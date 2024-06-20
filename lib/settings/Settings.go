package settings

import (
	"encoding/json"
	clientVars2 "github.com/ether/etherpad-go/lib/models/clientVars"
	"os"
	"reflect"
	"regexp"
)

type DBSettings struct {
	Filename string
}

type PadOptions struct {
	NoColors         bool
	ShowControls     bool
	ShowChat         bool
	ShowLineNumbers  bool
	UseMonospaceFont bool
	UserName         *bool
	UserColor        *bool
	RTL              bool
	AlwaysShowChat   bool
	ChatAndUsers     bool
	Lang             *string
}

type PadShortCutEnabled struct {
}

type Toolbar struct {
	Left       [][]string
	Right      [][]string
	TimeSlider [][]string
}

type User struct {
}

type Settings struct {
	Root                               *string
	Title                              string                                         `json:"title"`
	Favicon                            string                                         `json:"favicon"`
	SkinName                           string                                         `json:"skinName"`
	SkinVariants                       string                                         `json:"skinVariants"`
	IP                                 string                                         `json:"ip"`
	Port                               string                                         `json:"port"`
	SuppressErrorsInPadText            bool                                           `json:"suppressErrorsInPadText"`
	SSL                                bool                                           `json:"ssl"`
	DBType                             string                                         `json:"dbType"`
	DBSettings                         DBSettings                                     `json:"dbSettings"`
	DefaultPadText                     string                                         `json:"defaultPadText"`
	PadOptions                         PadOptions                                     `json:"padOptions"`
	PadShortCutEnabled                 PadShortCutEnabled                             `json:"padShortCutEnabled"`
	Toolbar                            Toolbar                                        `json:"toolbar"`
	RequireSession                     bool                                           `json:"requireSession"`
	EditOnly                           bool                                           `json:"editOnly"`
	MaxAge                             int                                            `json:"maxAge"`
	Minify                             bool                                           `json:"minify"`
	Abiword                            *string                                        `json:"abiword"`
	SOffice                            *string                                        `json:"soffice"`
	AllowUnknownFileEnds               bool                                           `json:"allowUnknownFileEnds"`
	LogLevel                           string                                         `json:"logLevel"`
	DisableIPLogging                   bool                                           `json:"disableIPLogging"`
	AutomaticReconnectionTimeout       int                                            `json:"automaticReconnectionTimeout"`
	LoadTest                           bool                                           `json:"loadTest"`
	DumpOnCleanExit                    bool                                           `json:"dumpOnCleanExit"`
	IndentationOnNewLine               bool                                           `json:"indentationOnNewLine"`
	SessionKey                         *string                                        `json:"SessionKey"`
	TrustProxy                         bool                                           `json:"trustProxy"`
	RequireAuthentication              bool                                           `json:"requireAuthentication"`
	RequireAuthorization               bool                                           `json:"requireAuthorization"`
	Users                              *User                                          `json:"users"`
	ShowSettingsInAdminPage            bool                                           `json:"showSettingsInAdminPage"`
	ScrollWhenFocusLineIsOutOfViewport clientVars2.ScrollWhenFocusLineIsOutOfViewport `json:"scrollWhenFocusLineIsOutOfViewport"`
}

var SettingsDisplayed Settings

func newDefaultSettings() Settings {
	return Settings{}
}

// Function to remove comments from JSON
func stripComments(jsonData []byte) []byte {
	// Define regex patterns to find and remove comments
	singleLineCommentRegex := regexp.MustCompile(`(?m)(^\s*//.*$|//.*$)`)
	multiLineCommentRegex := regexp.MustCompile(`(?s)/\*.*?\*/`)

	// Remove single-line comments
	jsonData = singleLineCommentRegex.ReplaceAll(jsonData, nil)
	// Remove multi-line comments
	jsonData = multiLineCommentRegex.ReplaceAll(jsonData, nil)
	re := regexp.MustCompile(`\r?\n|\r`)
	jsonData = re.ReplaceAll(jsonData, nil)

	return jsonData
}

// Merge merges non-zero values from src into dest, including nested structs, slices, and maps.
func Merge(dest, src interface{}) {
	destVal := reflect.ValueOf(dest).Elem()
	srcVal := reflect.ValueOf(src).Elem()

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		destField := destVal.Field(i)

		switch srcField.Kind() {
		case reflect.Struct:
			Merge(destField.Addr().Interface(), srcField.Addr().Interface())
		case reflect.Slice:
			if !srcField.IsNil() {
				destField.Set(reflect.AppendSlice(destField, srcField))
			}
		case reflect.Map:
			if !srcField.IsNil() {
				if destField.IsNil() {
					destField.Set(reflect.MakeMap(destField.Type()))
				}
				for _, key := range srcField.MapKeys() {
					destField.SetMapIndex(key, srcField.MapIndex(key))
				}
			}
		default:
			if !isZero(srcField) {
				destField.Set(srcField)
			}
		}
	}
}

// isZero reports whether v is the zero value for its type.
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Invalid:
		return true
	}
	return false
}

func init() {
	settings, err := os.ReadFile("settings.json")
	settings = stripComments(settings)
	SettingsDisplayed = newDefaultSettings()

	if err != nil {
		println("Error reading settings")
		return
	}
	var fileReadSettings Settings
	err = json.Unmarshal(settings, &fileReadSettings)
	Merge(&SettingsDisplayed, &fileReadSettings)

	if err != nil {
		println("error is" + err.Error())
		return
	}
}
