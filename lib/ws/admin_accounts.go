package ws

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
)

// AccountView is an account as the admin page lists it: never its
// password hash.
type AccountView struct {
	ID                 string           `json:"id"`
	Username           string           `json:"username"`
	DisplayName        string           `json:"displayName"`
	Email              string           `json:"email"`
	Kind               string           `json:"kind"`
	Disabled           bool             `json:"disabled"`
	MustChangePassword bool             `json:"mustChangePassword"`
	HasPassword        bool             `json:"hasPassword"`
	CreatedAt          int64            `json:"createdAt"`
	LastLoginAt        *int64           `json:"lastLoginAt"`
	Roles              []AssignmentView `json:"roles"`
	// Admin: holds the admin role's rights for all pads.
	Admin bool `json:"admin"`
}

type AssignmentView struct {
	Role      string `json:"role"`
	Pads      string `json:"pads"`
	CreatedBy string `json:"createdBy"`
	CreatedAt int64  `json:"createdAt"`
}

type RoleView struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Builtin     bool             `json:"builtin"`
	Scopes      []string         `json:"scopes"`
	Includes    []string         `json:"includes"`
	Limits      map[string]int64 `json:"limits"`
	// Modified: a built-in role that differs from how it was shipped.
	Modified bool `json:"modified"`
	// Fixed: cannot be changed at all (the admin role).
	Fixed bool `json:"fixed"`
}

type ScopeView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	PadScoped   bool   `json:"padScoped"`
}

// AccountsResult answers every account event with the whole state.
type AccountsResult struct {
	Accounts []AccountView `json:"accounts"`
	Roles    []RoleView    `json:"roles"`
	Scopes   []ScopeView   `json:"scopes"`
	// Me is the account of this admin connection.
	Me string `json:"me"`
	// Password is a new one-time password (accountCreate, accountResetPassword),
	// shown once.
	Password *OneTimePassword `json:"password,omitempty"`
	Error    string           `json:"error,omitempty"`
}

type OneTimePassword struct {
	AccountID string `json:"accountId"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

type AuditView struct {
	ID      string          `json:"id"`
	At      int64           `json:"at"`
	Actor   string          `json:"actor"`
	Action  string          `json:"action"`
	Target  string          `json:"target"`
	Details json.RawMessage `json:"details,omitempty"`
}

type AuditResult struct {
	Entries []AuditView `json:"entries"`
	// Names maps account ids in the entries to usernames.
	Names map[string]string `json:"names"`
	Error string            `json:"error,omitempty"`
}

func isAccountEvent(event string) bool {
	switch event {
	case "accountsLoad", "accountCreate", "accountUpdate", "accountResetPassword", "accountSetDisabled",
		"accountDelete", "assignmentAdd", "assignmentRemove", "roleSave", "roleDelete", "roleReset",
		"templateApply", "auditLoad":
		return true
	}
	return false
}

// accountErrors are shown as they are; other errors are logged.
var accountErrors = []error{
	accounts.ErrForbidden, accounts.ErrLastAdmin, accounts.ErrAdminRoleFixed, accounts.ErrBuiltinRole,
	accounts.ErrAnonymousScope, accounts.ErrRoleCycle, accounts.ErrEscalation, accounts.ErrInvalidUsername,
	accounts.ErrInvalidRole, accounts.ErrNoPassword, accounts.ErrAnonymousAccount, accounts.ErrInvalidProfile,
	accounts.ErrNotBuiltin, accounts.ErrPasswordTooShort, accounts.ErrBusy, accounts.ErrAuditFailed,
	db.ErrAccountNotFound, db.ErrRoleNotFound, db.ErrAssignmentNotFound,
}

// actor is the admin of the connection with its rights as they are now,
// not as they were at login: a disabled admin or one who lost rights
// cannot change anything more.
func (h AdminMessageHandler) actor(m *accounts.Manager, c *Client) (accounts.Actor, error) {
	if c.AdminAccount == "" {
		return accounts.Actor{}, accounts.ErrForbidden
	}
	a, err := m.Account(c.AdminAccount)
	if err != nil {
		return accounts.Actor{}, err
	}
	if a == nil || a.Kind != accounts.KindHuman || a.DisabledAt != nil {
		return accounts.Actor{}, accounts.ErrForbidden
	}
	g, err := m.GrantsFor(a.ID)
	if err != nil {
		return accounts.Actor{}, err
	}
	return accounts.Actor{ID: a.ID, Grants: g}, nil
}

// handleAccountEvent manages accounts, roles and the audit log. Account
// events are answered with "results:accounts", auditLoad with
// "results:audit".
func (h AdminMessageHandler) handleAccountEvent(event string, data json.RawMessage, c *Client) {
	m := accounts.For(h.store)
	actor, err := h.actor(m, c)
	if event == "auditLoad" {
		h.sendAudit(m, actor, err, data, c)
		return
	}
	result := AccountsResult{Me: c.AdminAccount}
	if err == nil {
		result.Password, err = h.applyAccountEvent(m, actor, event, data)
	}
	if err != nil {
		result.Error = h.accountError(event, err)
	}
	if err := h.fillAccounts(m, &result); err != nil {
		h.Logger.Errorf("admin: listing accounts: %v", err)
		if result.Error == "" {
			result.Error = "the accounts could not be loaded"
		}
	}
	h.sendResult(c, "results:accounts", result)
}

func (h AdminMessageHandler) accountError(event string, err error) string {
	for _, known := range accountErrors {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	if errors.Is(err, db.ErrConflict) {
		return "that name is already taken"
	}
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntax) || errors.As(err, &typeErr) {
		return "invalid request"
	}
	h.Logger.Errorf("admin: %s: %v", event, err)
	return "the change could not be made"
}

func (h AdminMessageHandler) applyAccountEvent(m *accounts.Manager, actor accounts.Actor, event string, data json.RawMessage) (*OneTimePassword, error) {
	var req struct {
		ID          string           `json:"id"`
		AccountID   string           `json:"accountId"`
		Username    *string          `json:"username"`
		DisplayName *string          `json:"displayName"`
		Email       *string          `json:"email"`
		Kind        string           `json:"kind"`
		Role        string           `json:"role"`
		Pads        string           `json:"pads"`
		Disabled    bool             `json:"disabled"`
		Template    string           `json:"template"`
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Scopes      []string         `json:"scopes"`
		Includes    []string         `json:"includes"`
		Limits      map[string]int64 `json:"limits"`
	}
	if event != "accountsLoad" && len(data) > 0 {
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
	}
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	switch event {
	case "accountCreate":
		n := accounts.NewAccount{Username: str(req.Username), DisplayName: str(req.DisplayName), Email: str(req.Email), Kind: req.Kind}
		var password string
		if n.Kind == accounts.KindHuman {
			password = accounts.RandomPassword()
			n.Password, n.MustChangePassword = password, true
		}
		if req.Role != "" {
			// Checked first, so an account is not left behind without
			// the role it was made for.
			roles, err := m.Roles()
			if err != nil {
				return nil, err
			}
			if _, ok := roles[req.Role]; !ok {
				return nil, db.ErrRoleNotFound
			}
		}
		a, err := m.CreateAccount(actor, n)
		if err != nil {
			return nil, err
		}
		h.Logger.Infof("admin: account %s created by %s", a.Username, actor.ID)
		if req.Role != "" {
			if err := m.Assign(actor, a.ID, req.Role, req.Pads); err != nil {
				// Not left behind without the role it was made for.
				if delErr := m.DeleteAccount(actor, a.ID); delErr != nil {
					h.Logger.Errorf("admin: removing account %s after a failed assignment: %v", a.ID, delErr)
				}
				return nil, err
			}
		}
		if password == "" {
			return nil, nil
		}
		return &OneTimePassword{AccountID: a.ID, Username: a.Username, Password: password}, nil
	case "accountUpdate":
		_, err := m.UpdateProfile(actor, req.ID, accounts.Profile{Username: req.Username, DisplayName: req.DisplayName, Email: req.Email})
		return nil, err
	case "accountResetPassword":
		password, err := m.ResetPassword(actor, req.ID)
		if err != nil && !errors.Is(err, accounts.ErrAuditFailed) {
			return nil, err
		}
		a, _ := m.Account(req.ID)
		otp := &OneTimePassword{AccountID: req.ID, Password: password}
		if a != nil {
			otp.Username = a.Username
		}
		h.Logger.Infof("admin: password of %s reset by %s", req.ID, actor.ID)
		return otp, err
	case "accountSetDisabled":
		return nil, m.SetDisabled(actor, req.ID, req.Disabled)
	case "accountDelete":
		err := m.DeleteAccount(actor, req.ID)
		if err == nil {
			h.Logger.Infof("admin: account %s deleted by %s", req.ID, actor.ID)
		}
		return nil, err
	case "assignmentAdd":
		return nil, m.Assign(actor, req.AccountID, req.Role, req.Pads)
	case "assignmentRemove":
		return nil, m.Unassign(actor, req.AccountID, req.Role, req.Pads)
	case "roleSave":
		_, err := m.SaveRole(actor, db.Role{ID: req.ID, Name: req.Name, Description: req.Description,
			Scopes: req.Scopes, Includes: req.Includes, Limits: req.Limits})
		return nil, err
	case "roleDelete":
		return nil, m.DeleteRole(actor, req.ID)
	case "roleReset":
		_, err := m.ResetRole(actor, req.ID)
		return nil, err
	case "templateApply":
		return nil, m.ApplyTemplate(actor, accounts.Template(req.Template))
	}
	return nil, nil
}

func (h AdminMessageHandler) fillAccounts(m *accounts.Manager, result *AccountsResult) error {
	list, err := m.Accounts()
	if err != nil {
		return err
	}
	assignments, err := m.Assignments()
	if err != nil {
		return err
	}
	roles, err := m.Roles()
	if err != nil {
		return err
	}
	byAccount := map[string][]AssignmentView{}
	for _, as := range assignments {
		byAccount[as.AccountID] = append(byAccount[as.AccountID], AssignmentView{
			Role: as.RoleID, Pads: as.PadPattern, CreatedBy: as.CreatedBy, CreatedAt: as.CreatedAt.UnixMilli()})
	}
	result.Accounts = make([]AccountView, 0, len(list))
	for _, a := range list {
		v := AccountView{ID: a.ID, Username: a.Username, DisplayName: a.DisplayName, Email: a.Email, Kind: a.Kind,
			Disabled: a.DisabledAt != nil, MustChangePassword: a.MustChangePassword, HasPassword: a.PasswordHash != "",
			CreatedAt: a.CreatedAt.UnixMilli(), Roles: byAccount[a.ID]}
		if v.Roles == nil {
			v.Roles = []AssignmentView{}
		}
		if a.LastLoginAt != nil {
			ms := a.LastLoginAt.UnixMilli()
			v.LastLoginAt = &ms
		}
		if a.Kind == accounts.KindHuman {
			v.Admin, _ = m.IsAdmin(a.ID)
		}
		result.Accounts = append(result.Accounts, v)
	}
	builtin := map[string]db.Role{}
	for _, r := range accounts.BuiltinRoles() {
		builtin[r.ID] = r
	}
	result.Roles = make([]RoleView, 0, len(roles))
	for _, r := range roles {
		v := RoleView{ID: r.ID, Name: r.Name, Description: r.Description, Builtin: r.Builtin,
			Scopes: nonNil(r.Scopes), Includes: nonNil(r.Includes), Limits: r.Limits, Fixed: r.ID == accounts.RoleAdmin}
		if v.Limits == nil {
			v.Limits = map[string]int64{}
		}
		if b, ok := builtin[r.ID]; ok && r.Builtin {
			v.Modified = !sameRole(b, r)
		}
		result.Roles = append(result.Roles, v)
	}
	sort.Slice(result.Roles, func(i, j int) bool {
		if result.Roles[i].Builtin != result.Roles[j].Builtin {
			return result.Roles[i].Builtin
		}
		return result.Roles[i].Name < result.Roles[j].Name
	})
	for _, s := range accounts.Scopes() {
		result.Scopes = append(result.Scopes, ScopeView{Name: s.Name, Description: s.Description, PadScoped: s.PadScoped})
	}
	return nil
}

func nonNil(s []string) []string {
	out := append([]string{}, s...)
	sort.Strings(out)
	return out
}

func sameRole(a, b db.Role) bool {
	norm := func(l map[string]int64) map[string]int64 {
		if len(l) == 0 {
			return nil
		}
		return l
	}
	return a.Name == b.Name && a.Description == b.Description &&
		reflect.DeepEqual(nonNil(a.Scopes), nonNil(b.Scopes)) &&
		reflect.DeepEqual(nonNil(a.Includes), nonNil(b.Includes)) &&
		reflect.DeepEqual(norm(a.Limits), norm(b.Limits))
}

// auditLimit caps the entries one auditLoad returns.
const auditLimit = 1000

func (h AdminMessageHandler) sendAudit(m *accounts.Manager, actor accounts.Actor, err error, data json.RawMessage, c *Client) {
	result := AuditResult{Entries: []AuditView{}, Names: map[string]string{}}
	if err == nil && !actor.Grants.HasGlobal(accounts.ScopeAdminUsers) && !actor.Grants.HasGlobal(accounts.ScopeAdminRoles) {
		err = accounts.ErrForbidden
	}
	var req struct {
		Limit int `json:"limit"`
	}
	if err == nil && len(data) > 0 {
		err = json.Unmarshal(data, &req)
	}
	if req.Limit <= 0 || req.Limit > auditLimit {
		req.Limit = auditLimit
	}
	var entries []db.AuditEntry
	if err == nil {
		entries, err = m.Audit(req.Limit)
	}
	if err != nil {
		result.Error = h.accountError("auditLoad", err)
		h.sendResult(c, "results:audit", result)
		return
	}
	if list, err := m.Accounts(); err == nil {
		for _, a := range list {
			result.Names[a.ID] = a.Username
		}
	}
	for _, e := range entries {
		// Deleted accounts keep the name they were created or deleted with.
		if _, known := result.Names[e.Target]; !known && (e.Action == "account.create" || e.Action == "account.delete") {
			var d struct {
				Username string `json:"username"`
			}
			if json.Unmarshal([]byte(e.Details), &d) == nil && d.Username != "" {
				result.Names[e.Target] = d.Username
			}
		}
	}
	for _, e := range entries {
		v := AuditView{ID: e.ID, At: e.At.UnixMilli(), Actor: e.ActorID, Action: e.Action, Target: e.Target}
		if e.Details != "" && json.Valid([]byte(e.Details)) {
			v.Details = json.RawMessage(e.Details)
		}
		result.Entries = append(result.Entries, v)
	}
	h.sendResult(c, "results:audit", result)
}

func (h AdminMessageHandler) sendResult(c *Client, event string, result any) {
	resp, err := json.Marshal([]any{event, result})
	if err != nil {
		h.Logger.Errorf("admin: encoding %s: %v", event, err)
		return
	}
	c.SafeSend(resp)
}
