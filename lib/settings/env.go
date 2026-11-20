package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Settings from the environment, in two forms:
//
//   - ETHERPAD_<PATH>, like Spring Boot's relaxed binding: the name is
//     matched against the fields of Settings, ignoring case and
//     underscores, so ETHERPAD_PADOPTIONS_SHOWCHAT and
//     ETHERPAD_PAD_OPTIONS_SHOW_CHAT both set padOptions.showChat, and a
//     number is a place in a list: ETHERPAD_SSO_CLIENTS_0_CLIENT_ID is the
//     client_id of the first SSO client.
//   - EP__a__b__c (etherpad-lite's form), with each part spelled out; the
//     only way into maps with free keys (users, plugins, plugin settings):
//     EP__users__alice__password. A number is a place in a list here too.
//
// A place in a list merges into the element from settings.json (only the
// named field changes; Spring Boot would replace the whole list); the
// place after the last one appends an element. A gap (0 and 2 but no 1)
// stops the start, so does a value that does not fit its field (a bool
// that is not true or false). A name that matches no setting is reported.

// envNotSettings are ETHERPAD_ variables that are not settings.
var envNotSettings = map[string]bool{
	SettingsFileEnv: true, SettingsPathEnv: true, "ETHERPAD_VERSION_STRING": true,
	"ETHERPAD_URL": true, "ETHERPAD_API_KEY": true,
}

// notASetting reports ETHERPAD_ variables of others: the CLI's, and the
// ones Kubernetes sets for a Service named etherpad (service links).
func notASetting(name string) bool {
	if envNotSettings[name] || strings.HasPrefix(name, "ETHERPAD_SERVICE_") {
		return true
	}
	// ETHERPAD_PORT_9001_TCP, ..._TCP_ADDR, ..._TCP_PORT, ..._TCP_PROTO
	if rest, ok := strings.CutPrefix(name, "ETHERPAD_PORT_"); ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
		return true
	}
	return false
}

// envSeg is one step of a settings path: a key, or a place in a list.
type envSeg struct {
	key   string
	index int
	// list: a place in a list; maybeList: a number below a value of any
	// type, a place only if the value there is a list.
	list, maybeList bool
}

type envOverride struct {
	name  string // the variable, for messages
	path  []envSeg
	value any
}

// EnvReport is what applyEnv did not use: names that match no setting.
type EnvReport struct {
	Unknown []string
}

var settingsType = reflect.TypeOf(Settings{})

// applyEnv applies the settings of environ (NAME=value) on top of what
// viper has read, and reports the names it could not use.
func applyEnv(environ []string) (EnvReport, error) {
	var report EnvReport
	registered := map[string]string{}
	for _, c := range Registry {
		registered[EnvVar(c.Key)] = c.Key
	}
	var overrides []envOverride
	for _, kv := range environ {
		name, raw, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		var path []envSeg
		var leaf reflect.Type
		switch {
		case strings.HasPrefix(name, "EP__"):
			parts := strings.Split(strings.TrimPrefix(name, "EP__"), "__")
			if slicesContain(parts, "") {
				report.Unknown = append(report.Unknown, name)
				continue
			}
			path, leaf, ok = resolveSpelled(settingsType, parts, true)
		case strings.HasPrefix(name, envPrefix+"_"):
			if notASetting(name) {
				continue
			}
			tokens := strings.Split(strings.TrimPrefix(name, envPrefix+"_"), "_")
			if slicesContain(tokens, "") {
				ok = false
			} else {
				path, leaf, ok = resolveRelaxed(settingsType, tokens)
			}
			if key, isRegistered := registered[name]; !ok && isRegistered {
				// A registered key that is no field of Settings, such as a
				// plugin's enabled flag (plugins is a map).
				path, leaf, ok = nil, nil, true
				for _, k := range strings.Split(key, ".") {
					path = append(path, envSeg{key: k})
				}
			}
		default:
			continue
		}
		if !ok {
			report.Unknown = append(report.Unknown, name)
			continue
		}
		value, keep, err := envValue(raw, leaf)
		if err != nil {
			return report, fmt.Errorf("%s: %w", name, err)
		}
		if !keep {
			continue
		}
		overrides = append(overrides, envOverride{name: name, path: path, value: value})
	}
	// In path order, places in a list by number: 0 before 1 before 10,
	// so a list grows one place at a time.
	sort.SliceStable(overrides, func(i, j int) bool { return pathLess(overrides[i].path, overrides[j].path) })
	for _, o := range overrides {
		if err := setEnvOverride(o); err != nil {
			return report, fmt.Errorf("%s: %w", o.name, err)
		}
	}
	sort.Strings(report.Unknown)
	return report, nil
}

func slicesContain(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func pathLess(a, b []envSeg) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		x, y := a[i], b[i]
		xn, xNum := segNumber(x)
		yn, yNum := segNumber(y)
		switch {
		case xNum && yNum && xn != yn:
			return xn < yn
		case xNum != yNum:
			return xNum
		case !xNum && !strings.EqualFold(x.key, y.key):
			return strings.ToLower(x.key) < strings.ToLower(y.key)
		}
	}
	return len(a) < len(b)
}

func segNumber(s envSeg) (int, bool) {
	if s.list {
		return s.index, true
	}
	if s.maybeList {
		n, err := strconv.Atoi(s.key)
		return n, err == nil
	}
	return 0, false
}

// field is a settings field as viper and mapstructure name it.
type field struct {
	name string
	typ  reflect.Type
}

// fieldsOf are the fields of a struct type, embedded and squashed ones
// flattened, "-" left out.
func fieldsOf(t reflect.Type) []field {
	var out []field
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("mapstructure")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if (f.Anonymous && name == "") || strings.Contains(opts, "squash") {
			ft := deref(f.Type)
			if ft.Kind() == reflect.Struct {
				out = append(out, fieldsOf(ft)...)
				continue
			}
		}
		if name == "" {
			name = f.Name
		}
		out = append(out, field{name: name, typ: f.Type})
	}
	return out
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// relaxed is a name for relaxed matching: lower case, no "_" or "-".
func relaxed(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(s))
}

func isNumber(s string) bool {
	if s == "" || len(s) > 9 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// resolveRelaxed matches the parts of an ETHERPAD_ name against t: a field
// may take several parts (CLIENT_ID is client_id), the longest match is
// tried first, then shorter ones. It returns the path and the type of the
// value it ends at.
func resolveRelaxed(t reflect.Type, tokens []string) ([]envSeg, reflect.Type, bool) {
	t = deref(t)
	if len(tokens) == 0 {
		return nil, t, true
	}
	switch t.Kind() {
	case reflect.Struct:
		fields := fieldsOf(t)
		for j := len(tokens); j >= 1; j-- {
			want := relaxed(strings.Join(tokens[:j], ""))
			for _, f := range fields {
				if relaxed(f.name) != want {
					continue
				}
				if rest, leaf, ok := resolveRelaxed(f.typ, tokens[j:]); ok {
					return append([]envSeg{{key: f.name}}, rest...), leaf, true
				}
			}
		}
	case reflect.Slice, reflect.Array:
		if isNumber(tokens[0]) {
			n, _ := strconv.Atoi(tokens[0])
			if rest, leaf, ok := resolveRelaxed(t.Elem(), tokens[1:]); ok {
				return append([]envSeg{{index: n, list: true}}, rest...), leaf, true
			}
		}
	}
	// Maps (free keys) and values of any type need EP__ with the parts
	// spelled out.
	return nil, nil, false
}

// resolveSpelled follows the parts of an EP__ name through t. Below a
// value of any type (plugin settings) it goes on without a type. top: an
// unknown top-level key is kept (etherpad-lite's plugin sections).
func resolveSpelled(t reflect.Type, parts []string, top bool) ([]envSeg, reflect.Type, bool) {
	if len(parts) == 0 {
		if t != nil {
			t = deref(t)
		}
		return nil, t, true
	}
	if t == nil || deref(t).Kind() == reflect.Interface {
		// No type: a number is a place only if there is a list.
		var path []envSeg
		for _, p := range parts {
			path = append(path, envSeg{key: p, maybeList: isNumber(p)})
		}
		return path, nil, true
	}
	t = deref(t)
	switch t.Kind() {
	case reflect.Struct:
		for _, f := range fieldsOf(t) {
			if relaxed(f.name) == relaxed(parts[0]) {
				rest, leaf, ok := resolveSpelled(f.typ, parts[1:], false)
				if !ok {
					return nil, nil, false
				}
				return append([]envSeg{{key: f.name}}, rest...), leaf, true
			}
		}
		if top {
			return resolveSpelled(nil, parts, false)
		}
	case reflect.Map:
		rest, leaf, ok := resolveSpelled(t.Elem(), parts[1:], false)
		if !ok {
			return nil, nil, false
		}
		return append([]envSeg{{key: parts[0]}}, rest...), leaf, true
	case reflect.Slice, reflect.Array:
		if isNumber(parts[0]) {
			n, _ := strconv.Atoi(parts[0])
			rest, leaf, ok := resolveSpelled(t.Elem(), parts[1:], false)
			if !ok {
				return nil, nil, false
			}
			return append([]envSeg{{index: n, list: true}}, rest...), leaf, true
		}
	}
	return nil, nil, false
}

// envValue types raw for a field of type t (nil: unknown, typed like
// ${VAR} values). keep is false for "undefined" (leave the setting).
func envValue(raw string, t reflect.Type) (any, bool, error) {
	// etherpad-lite's markers mean the same for every setting, whatever its
	// type: "undefined" leaves it as it is, "null" is null (as before: a
	// value from settings.json stays, viper falls back to it).
	switch raw {
	case "undefined":
		return nil, false, nil
	case "null":
		return nil, true, nil
	}
	if t == nil {
		v, keep := coerceEnvValue(raw)
		return v, keep, nil
	}
	switch t.Kind() {
	case reflect.String:
		return raw, true, nil
	case reflect.Bool:
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true":
			return true, true, nil
		case "false":
			return false, true, nil
		}
		return nil, false, fmt.Errorf("%q is not true or false", raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return nil, false, fmt.Errorf("%q is not a whole number", raw)
		}
		return n, true, nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return nil, false, fmt.Errorf("%q is not a number", raw)
		}
		return f, true, nil
	case reflect.Slice, reflect.Array:
		// A whole list of plain values at once: comma separated, as in
		// Spring Boot (ETHERPAD_AVAILABLEEXPORTS=pdf,html).
		elem := deref(t.Elem())
		if k := elem.Kind(); k == reflect.Struct || k == reflect.Map || k == reflect.Slice || k == reflect.Interface {
			return nil, false, fmt.Errorf("a list of %s is set place by place (…_0_…)", elem)
		}
		out := []any{}
		if strings.TrimSpace(raw) == "" {
			return out, true, nil
		}
		for _, part := range strings.Split(raw, ",") {
			v, _, err := envValue(strings.TrimSpace(part), elem)
			if err != nil {
				return nil, false, err
			}
			out = append(out, v)
		}
		return out, true, nil
	case reflect.Struct, reflect.Map:
		return nil, false, fmt.Errorf("names a whole section; set its fields")
	}
	v, keep := coerceEnvValue(raw)
	return v, keep, nil
}

// setEnvOverride puts an override into viper: a path without a list
// directly, one through a list by reading the list, changing a copy and
// setting it back whole (viper cannot address places in lists).
func setEnvOverride(o envOverride) error {
	for i, s := range o.path {
		isList := s.list
		if s.maybeList {
			// A number below a value of any type: a place in a list if
			// there is a list or nothing yet, a key in a section.
			switch viper.Get(joinKeys(o.path[:i])).(type) {
			case []any, nil:
				isList = true
			}
		}
		if !isList {
			continue
		}
		key := joinKeys(o.path[:i])
		updated, err := applyAt(deepCopy(viper.Get(key)), o.path[i:], o.value)
		if err != nil {
			return err
		}
		viper.Set(key, updated)
		return nil
	}
	viper.Set(joinKeys(o.path), o.value)
	return nil
}

func joinKeys(path []envSeg) string {
	keys := make([]string, len(path))
	for i, s := range path {
		keys[i] = s.key
	}
	return strings.Join(keys, ".")
}

// applyAt sets value at path below node (a value read from viper: maps,
// lists, plain values) and returns the changed node.
func applyAt(node any, path []envSeg, value any) (any, error) {
	if len(path) == 0 {
		return value, nil
	}
	s := path[0]
	isList := s.list
	index := s.index
	if s.maybeList {
		switch node.(type) {
		case []any, nil:
			isList, index = true, mustAtoi(s.key)
		}
	}
	if isList {
		if m, ok := node.(map[string]any); ok && len(m) == 0 {
			node = nil // an empty section: no list yet
		}
		list, ok := node.([]any)
		if node != nil && !ok {
			return nil, fmt.Errorf("%v is not a list", describe(node))
		}
		switch {
		case index < len(list):
			v, err := applyAt(list[index], path[1:], value)
			if err != nil {
				return nil, err
			}
			list[index] = v
		case index == len(list):
			v, err := applyAt(nil, path[1:], value)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		default:
			return nil, fmt.Errorf("place %d of a list with %d places: places are numbered without gaps from 0", index, len(list))
		}
		return list, nil
	}
	m, ok := node.(map[string]any)
	if node != nil && !ok {
		return nil, fmt.Errorf("%v has no field %s", describe(node), s.key)
	}
	if m == nil {
		m = map[string]any{}
	}
	key := s.key
	for k := range m {
		if strings.EqualFold(k, s.key) {
			key = k
			break
		}
	}
	v, err := applyAt(m[key], path[1:], value)
	if err != nil {
		return nil, err
	}
	m[key] = v
	return m, nil
}

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func describe(v any) string {
	switch v.(type) {
	case []any:
		return "a list"
	case map[string]any:
		return "a section"
	}
	return fmt.Sprintf("the value %v", v)
}

// deepCopy copies maps and lists, so a value read from viper is not changed
// in place.
func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[k] = deepCopy(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = deepCopy(x)
		}
		return out
	case []map[string]any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = deepCopy(x)
		}
		return out
	case nil:
		return nil
	}
	// Typed values (defaults such as []string or a slice of structs) as
	// viper would read them from JSON.
	if k := reflect.TypeOf(v).Kind(); k == reflect.Slice || k == reflect.Map || k == reflect.Struct || k == reflect.Pointer {
		if data, err := json.Marshal(v); err == nil {
			var out any
			if json.Unmarshal(data, &out) == nil {
				return out
			}
		}
	}
	return v
}

// reportEnv tells about variables that set nothing.
func reportEnv(r EnvReport) {
	for _, name := range r.Unknown {
		fmt.Fprintf(os.Stderr, "warning: %s matches no setting and is ignored (ETHERPAD_<PATH> for fields, "+
			"places in lists as numbers; EP__a__b for maps such as users or plugins)\n", name)
	}
}

// viperGet reads a setting by its path (tests, the CLI).
func viperGet(key string) any { return viper.Get(key) }
