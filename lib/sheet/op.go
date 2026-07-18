package sheet

import (
	"fmt"
	"regexp"
	"strconv"
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
	// Sheet-list ops: Op.Sheet names the target sheet id.
	OpAddSheet    OpType = "addSheet"
	OpRenameSheet OpType = "renameSheet"
	OpDeleteSheet OpType = "deleteSheet"
	OpMoveSheet   OpType = "moveSheet"
	// Grid metadata ops.
	OpSetDimension OpType = "setDimension"
	OpSetFreeze    OpType = "setFreeze"
	// Merged cells: both carry the Row/Col..EndRow/EndCol rectangle.
	OpMergeCells   OpType = "mergeCells"
	OpUnmergeCells OpType = "unmergeCells"
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

	// Structural ops (insert/delete rows/cols). Index doubles as the insertion
	// position for addSheet.
	Index int `json:"index,omitempty"`
	Count int `json:"count,omitempty"`

	// Sheet-list ops.
	Name    string `json:"name,omitempty"`    // addSheet, renameSheet
	ToIndex int    `json:"toIndex,omitempty"` // moveSheet

	// setDimension.
	Axis string `json:"axis,omitempty"` // "col" or "row"
	Size int    `json:"size,omitempty"` // px

	// setFreeze. 0 or 1 each (freeze first row / first col only for now).
	FrozenRows int `json:"frozenRows,omitempty"`
	FrozenCols int `json:"frozenCols,omitempty"`
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
		if err := ValidateProps(o.Props); err != nil {
			return err
		}
	case OpSetStyle:
		if o.StyleId == nil && o.Props == nil {
			return fmt.Errorf("setStyle needs styleId or props")
		}
		if o.Row < 0 || o.Col < 0 {
			return fmt.Errorf("setStyle negative coord")
		}
		if err := ValidateProps(o.Props); err != nil {
			return err
		}
	// mergeCells allows a degenerate 1x1 rectangle (Apply no-ops it): rebasing
	// past a concurrent row/col delete can collapse a valid merge to one cell.
	case OpClearRange, OpMergeCells, OpUnmergeCells:
		if o.Row < 0 || o.Col < 0 || o.EndRow < o.Row || o.EndCol < o.Col {
			return fmt.Errorf("%s invalid bounds", o.Type)
		}
	case OpInsertRows, OpDeleteRows, OpInsertCols, OpDeleteCols:
		if o.Index < 0 {
			return fmt.Errorf("%s negative index", o.Type)
		}
		if o.Count <= 0 {
			return fmt.Errorf("%s count must be > 0", o.Type)
		}
	case OpAddSheet, OpRenameSheet:
		if o.Name == "" {
			return fmt.Errorf("%s needs a name", o.Type)
		}
		if len(o.Name) > 128 {
			return fmt.Errorf("%s name too long", o.Type)
		}
		if o.Index < 0 {
			return fmt.Errorf("%s negative index", o.Type)
		}
	case OpDeleteSheet:
		// Last-sheet protection is stateful and enforced in Apply.
	case OpMoveSheet:
		if o.ToIndex < 0 {
			return fmt.Errorf("moveSheet negative toIndex")
		}
	case OpSetDimension:
		if o.Axis != "col" && o.Axis != "row" {
			return fmt.Errorf("setDimension axis must be col or row")
		}
		if o.Index < 0 {
			return fmt.Errorf("setDimension negative index")
		}
		if o.Size <= 0 || o.Size > 4096 {
			return fmt.Errorf("setDimension size out of range")
		}
	case OpSetFreeze:
		if o.FrozenRows < 0 || o.FrozenRows > 1 || o.FrozenCols < 0 || o.FrozenCols > 1 {
			return fmt.Errorf("setFreeze supports only 0 or 1 frozen rows/cols")
		}
	default:
		return fmt.Errorf("unknown op type %q", o.Type)
	}
	return nil
}

var (
	hexColorRe = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	numFmtRe   = regexp.MustCompile(`^(general|text|date|(number|currency|percent)(:\d{1,2})?)$`)
	fontSizeRe = regexp.MustCompile(`^[1-9]\d?$`) // 1-2 digits, no leading zeros; range-checked below
)

var fontFamilies = map[string]bool{
	"Calibri": true, "Arial": true, "Times New Roman": true,
	"Courier New": true, "Georgia": true, "Verdana": true,
}

// ValidateProps allowlists style prop keys and values. Props come from
// arbitrary collaborators and end up as inline CSS on every viewer's DOM, so
// anything outside the known vocabulary is rejected (e.g. bg: "url(...)").
func ValidateProps(props map[string]string) error {
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
		case "fontFamily":
			ok = fontFamilies[v]
		case "fontSize":
			if fontSizeRe.MatchString(v) {
				n, _ := strconv.Atoi(v)
				ok = n >= 6 && n <= 96
			}
		case "wrap":
			ok = v == "1"
		default:
			return fmt.Errorf("props: unknown key %q", k)
		}
		if !ok {
			return fmt.Errorf("props: invalid value %q for %q", v, k)
		}
	}
	return nil
}
