package compact

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
)

const usage = `Usage: etherpad-go compact [options] <padID>...
       etherpad-go compact [options] --all
       etherpad-go compact [options] --older-than <days>

Compacts the revision history of pads to reclaim database space, through
the HTTP API of the running server (etherpad-lite's bin/compactPad,
compactAllPads and compactStalePads). Destructive: export what you cannot
lose as .etherpad first.

Options:
`

// RunFromCLI runs "etherpad-go compact" and exits: 0 when every pad was
// compacted, 1 when one failed, 2 on a usage error.
func RunFromCLI(args []string) {
	os.Exit(Main(args, os.Stdout, os.Stderr, nil))
}

// Main is RunFromCLI without exiting; api replaces the HTTP API in tests.
func Main(args []string, out, errOut io.Writer, api API) int {
	return mainWithClock(args, out, errOut, api, time.Now)
}

func mainWithClock(args []string, out, errOut io.Writer, api API, now func() time.Time) int {
	fs := flag.NewFlagSet("compact", flag.ContinueOnError)
	fs.SetOutput(errOut)
	all := fs.Bool("all", false, "compact every pad of the instance")
	olderThan := fs.Int("older-than", 0, "compact only pads not edited for this many `days`")
	var keep optionalInt
	fs.Var(&keep, "keep", "keep the last `N` revisions (default: collapse all history)")
	dryRun := fs.Bool("dry-run", false, "list the pads and their revisions, change nothing")
	baseURL := fs.String("url", "", "the server's `URL` (default: ETHERPAD_URL, else from the settings: http://<ip>:<port>; a reverse proxy's https address works too)")
	apiKey := fs.String("apikey", "", "a token (admin page: Tokens) that may compact the pads (default: ETHERPAD_API_KEY)")
	fs.Usage = func() {
		fmt.Fprint(errOut, usage)
		fs.PrintDefaults()
	}
	// Flags may follow the pad ids: "compact myPad --keep 10".
	var pads []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		pads = append(pads, fs.Arg(0))
		rest = fs.Args()[1:]
	}

	selectors := 0
	for _, set := range []bool{len(pads) > 0, *all, *olderThan > 0} {
		if set {
			selectors++
		}
	}
	if selectors != 1 {
		fmt.Fprintln(errOut, "Give pad ids, --all or --older-than <days> (one of them).")
		fs.Usage()
		return 2
	}
	if *olderThan < 0 || *olderThan > MaxOlderThanDays {
		fmt.Fprintf(errOut, "--older-than must be between 1 and %d days.\n", MaxOlderThanDays)
		return 2
	}
	opts := Options{Pads: pads, OlderThanDays: *olderThan, DryRun: *dryRun, Keep: keep.value}

	if api == nil {
		key := *apiKey
		if key == "" {
			key = os.Getenv("ETHERPAD_API_KEY")
		}
		if key == "" {
			fmt.Fprintln(errOut, "A token is needed: --apikey or ETHERPAD_API_KEY (issue one on the admin page, Tokens).")
			return 2
		}
		u := *baseURL
		if u == "" {
			u = os.Getenv("ETHERPAD_URL")
		}
		if u == "" {
			u = urlFromSettings()
		}
		api = &HTTPAPI{BaseURL: u, APIKey: key}
	}

	r := Run(api, opts, out, errOut, now)
	if r.Failed > 0 {
		return 1
	}
	return 0
}

// optionalInt is a non-negative int flag that may be left out.
type optionalInt struct{ value *int }

func (o *optionalInt) String() string {
	if o.value == nil {
		return ""
	}
	return strconv.Itoa(*o.value)
}

func (o *optionalInt) Set(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fmt.Errorf("must be a non-negative number of revisions")
	}
	o.value = &n
	return nil
}

// urlFromSettings is the server's address as its settings give it. The
// server itself always speaks plain HTTP (TLS is a reverse proxy's job); give
// --url for the proxy's https address.
func urlFromSettings() string {
	ip, port := "localhost", "9001"
	if s, err := settings.ReadConfig(); err == nil {
		if s.IP != "" && s.IP != "0.0.0.0" && s.IP != "::" {
			ip = s.IP
		}
		if s.Port != "" {
			port = s.Port
		}
	}
	if strings.Contains(ip, ":") && !strings.HasPrefix(ip, "[") {
		ip = "[" + ip + "]"
	}
	return "http://" + ip + ":" + port
}
