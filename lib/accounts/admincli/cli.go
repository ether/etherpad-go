// Package admincli is "etherpad-go admin": creating admins, resetting
// passwords and listing accounts from the command line, on the database of
// the settings. It is the way into an instance without an admin, and back
// in when every admin is locked out.
package admincli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"go.uber.org/zap"
)

const usage = `Usage: etherpad-go admin <command> [options]

Commands:
  create <username> [--role <role>] [--pads <pattern>]
        Create a person with a one-time password, shown once; it must be
        changed at the first login. The role is admin unless given.
  reset-password <username>
        Give a person a new one-time password and enable the account.
  list  List the accounts with their roles.

The commands work on the database of the settings (as the server reads
them), also while the server runs. Every change is written to the audit
log as done by "cli".
`

// RunFromCLI runs "etherpad-go admin" and exits: 0 on success, 1 on an
// error, 2 on a usage error.
func RunFromCLI(logger *zap.SugaredLogger, args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	settings.InitSettings(logger)
	cfg := settings.Displayed
	if cfg.DBType == settings.MEMORY {
		fmt.Fprintln(os.Stderr, "The settings use the in-memory database: accounts made here would be gone at once. Configure SQLite, MySQL or Postgres.")
		os.Exit(1)
	}
	store, err := utils.GetDB(cfg, zap.NewNop().Sugar())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open the database: %v\n", err)
		os.Exit(1)
	}
	os.Exit(Main(args, os.Stdout, os.Stderr, store, &cfg))
}

// Main is RunFromCLI on an open store, without exiting.
func Main(args []string, out, errOut io.Writer, store db.DataStore, cfg *settings.Settings) int {
	if len(args) == 0 {
		fmt.Fprint(errOut, usage)
		return 2
	}
	m := accounts.For(store)
	if err := m.Ensure(cfg, nil); err != nil {
		fmt.Fprintf(errOut, "Cannot set up the accounts: %v\n", err)
		return 1
	}
	switch args[0] {
	case "create":
		return create(m, args[1:], out, errOut)
	case "reset-password":
		return resetPassword(m, args[1:], out, errOut)
	case "list":
		return list(m, out, errOut)
	}
	fmt.Fprintf(errOut, "Unknown command %q.\n\n%s", args[0], usage)
	return 2
}

// parse reads one username and the flags, in any order.
func parse(fs *flag.FlagSet, args []string) (string, error) {
	var names []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return "", err
		}
		if fs.NArg() == 0 {
			break
		}
		names = append(names, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	if len(names) != 1 {
		return "", errors.New("give exactly one username")
	}
	return names[0], nil
}

func printPassword(out io.Writer, username, password string) {
	fmt.Fprintf(out, "Username: %s\nOne-time password: %s\n\nLog in with it at /admin/; a new password must be set right away. It is not shown again.\n", username, password)
}

func create(m *accounts.Manager, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	fs.SetOutput(errOut)
	role := fs.String("role", accounts.RoleAdmin, "the `role` to give")
	pads := fs.String("pads", "", "limit the role to the pads matching this `pattern` (default: all pads)")
	username, err := parse(fs, args)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(errOut, "%v\n\n%s", err, usage)
		}
		return 2
	}
	roles, err := m.Roles()
	if err != nil {
		fmt.Fprintf(errOut, "Cannot read the roles: %v\n", err)
		return 1
	}
	roleID := *role
	if _, ok := roles[roleID]; !ok {
		// By name, as people see it.
		for id, r := range roles {
			if strings.EqualFold(r.Name, *role) {
				roleID = id
			}
		}
	}
	if _, ok := roles[roleID]; !ok {
		fmt.Fprintf(errOut, "There is no role %q.\n", *role)
		return 1
	}
	if err := accounts.ValidatePattern(*pads); err != nil {
		fmt.Fprintf(errOut, "Invalid pad pattern %q.\n", *pads)
		return 1
	}
	password := accounts.RandomPassword()
	a, err := m.CreateAccount(accounts.CLI, accounts.NewAccount{
		Username: username, Kind: accounts.KindHuman, Password: password, MustChangePassword: true,
	})
	switch {
	case errors.Is(err, db.ErrConflict):
		fmt.Fprintf(errOut, "The account %q exists already; use \"etherpad-go admin reset-password %s\" for a new password.\n", username, username)
		return 1
	case err != nil:
		fmt.Fprintf(errOut, "Cannot create the account: %v\n", err)
		return 1
	}
	if err := m.Assign(accounts.CLI, a.ID, roleID, *pads); err != nil {
		fmt.Fprintf(errOut, "Created %q, but cannot give it the role: %v\n", username, err)
		return 1
	}
	printPassword(out, username, password)
	return 0
}

func resetPassword(m *accounts.Manager, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("admin reset-password", flag.ContinueOnError)
	fs.SetOutput(errOut)
	username, err := parse(fs, args)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(errOut, "%v\n\n%s", err, usage)
		}
		return 2
	}
	a, err := m.AccountByUsername(username)
	if err != nil {
		fmt.Fprintf(errOut, "Cannot read the accounts: %v\n", err)
		return 1
	}
	if a == nil {
		fmt.Fprintf(errOut, "There is no account %q.\n", username)
		return 1
	}
	password, err := m.ResetPassword(accounts.CLI, a.ID)
	if err != nil {
		fmt.Fprintf(errOut, "Cannot reset the password: %v\n", err)
		return 1
	}
	printPassword(out, username, password)
	fmt.Fprintln(out, "The account is enabled. A login lock from failed attempts ends at most 15 minutes after the last one.")
	return 0
}

func list(m *accounts.Manager, out, errOut io.Writer) int {
	all, err := m.Accounts()
	if err != nil {
		fmt.Fprintf(errOut, "Cannot read the accounts: %v\n", err)
		return 1
	}
	roles, err := m.Roles()
	if err != nil {
		fmt.Fprintf(errOut, "Cannot read the roles: %v\n", err)
		return 1
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "USERNAME\tKIND\tSTATE\tROLES")
	for _, a := range all {
		refs, err := m.Refs(a.ID)
		if err != nil {
			fmt.Fprintf(errOut, "Cannot read the roles of %q: %v\n", a.Username, err)
			return 1
		}
		var names []string
		for _, r := range refs {
			name := r.Role
			if role, ok := roles[r.Role]; ok {
				name = role.Name
			}
			if r.Pads != "" {
				name += " (" + r.Pads + ")"
			}
			names = append(names, name)
		}
		state := "active"
		switch {
		case a.DisabledAt != nil:
			state = "disabled"
		case a.Kind == accounts.KindHuman && a.PasswordHash == "":
			state = "no password"
		case a.MustChangePassword:
			state = "must change password"
		}
		if a.Kind == accounts.KindBuiltin {
			state = "-"
		}
		roleList := strings.Join(names, ", ")
		if roleList == "" {
			roleList = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.Username, a.Kind, state, roleList)
	}
	_ = w.Flush()
	return 0
}
