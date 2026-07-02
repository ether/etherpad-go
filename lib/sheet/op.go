package sheet

import (
	"fmt"
	"regexp"
)

type OpType string

const (
	OpSetCell    OpType = "setCell"
	OpSetStyle   OpType = "setStyle"
	OpClearRange OpType = "clearRange"
	OpInsertRows OpType = "insertRows"
	OpDeleteRows OpType = "deleteRows"
	OpInsertCols OpType = "insertCols"
	OpDeleteCols OpType = "deleteCols"
)

// Op is one cell-based operation. BaseRev is the workbook revision the client
// composed it against (used by the server to rebase stale ops). Payload fields
// are optional per type.
type Op struct {
	Type    OpType `json:"type"`
	Sheet   string `json:"sheet"`
	BaseRev int    `json:"baseRev"`

	// Cell ops (setCell, setStyle) and the top-left of a range (clearRange).
	Row int `json:"row,omitempty"`
	Col int `json:"col,omitempty"`
	// Range end (inclusive) for clearRange.
	EndRow int `json:"endRow,omitempty"`
	EndCol int `json:"endCol,omitempty"`

	// setCell payload (pointers so "unset" is distinguishable from empty).
	Raw       *string `json:"raw,omitempty"`
	Value     *string `json:"value,omitempty"`
	ValueType *string `json:"valueType,omitempty"`
	// setCell + setStyle payload.
	StyleId *int `json:"styleId,omitempty"`
	// setCell + setStyle: style properties to intern into the workbook StylePool.
	// When present, Apply interns them and sets the cell's StyleId to the result.
	Props map[string]string `json:"props,omitempty"`

	// Structural ops (insert/delete rows/cols).
	Index int `json:"index,omitempty"`
	Count int `json:"count,omitempty"`
}

func (o Op) isStructural() bool {
	switch o.Type {
	case OpInsertRows, OpDeleteRows, OpInsertCols, OpDeleteCols:
		return true
	}
	return false
}

// Validate checks structural invariants independent of any workbook state.
func (o Op) Validate() error {
	if o.Sheet == "" {
		return fmt.Errorf("op missing sheet id")
	}
	switch o.Type {
	case OpSetCell:
		if o.Raw == nil && o.StyleId == nil && o.Props == nil {
			return fmt.Errorf("setCell needs raw, styleId, and/or props")
		}
		if o.Row < 0 || o.Col < 0 {
			return fmt.Errorf("setCell negative coord")
		}
		if err := validateProps(o.Props); err != nil {
			return err
		}
	case OpSetStyle:
		if o.StyleId == nil && o.Props == nil {
			return fmt.Errorf("setStyle needs styleId or props")
		}
		if o.Row < 0 || o.Col < 0 {
			return fmt.Errorf("setStyle negative coord")
		}
		if err := validateProps(o.Props); err != nil {
			return err
		}
	case OpClearRange:
		if o.Row < 0 || o.Col < 0 || o.EndRow < o.Row || o.EndCol < o.Col {
			return fmt.Errorf("clearRange invalid bounds")
		}
	case OpInsertRows, OpDeleteRows, OpInsertCols, OpDeleteCols:
		if o.Index < 0 {
			return fmt.Errorf("%s negative index", o.Type)
		}
		if o.Count <= 0 {
			return fmt.Errorf("%s count must be > 0", o.Type)
		}
	default:
		return fmt.Errorf("unknown op type %q", o.Type)
	}
	return nil
}

var (
	hexColorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	numFmtRe   = regexp.MustCompile(`^(general|text|date|(number|currency|percent)(:\d{1,2})?)$`)
)

// validateProps allowlists style prop keys and values. Props come from
// arbitrary collaborators and end up as inline CSS on every viewer's DOM, so
// anything outside the known vocabulary is rejected (e.g. bg: "url(...)").
func validateProps(props map[string]string) error {
	for k, v := range props {
		ok := false
		switch k {
		case "bold", "italic", "underline":
			ok = v == "1"
		case "color", "bg":
			ok = hexColorRe.MatchString(v)
		case "align":
			ok = v == "left" || v == "center" || v == "right"
		case "border":
			ok = v == "all"
		case "numFmt":
			ok = numFmtRe.MatchString(v)
		default:
			return fmt.Errorf("props: unknown key %q", k)
		}
		if !ok {
			return fmt.Errorf("props: invalid value %q for %q", v, k)
		}
	}
	return nil
}
