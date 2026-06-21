package sheet

// CellRef is a zero-based (row, col) address within a single sheet.
// It is a comparable struct so it can be used directly as a map key.
type CellRef struct {
	Row int
	Col int
}

// CellKind classifies a cell's raw content.
type CellKind string

const (
	KindValue   CellKind = "value"
	KindFormula CellKind = "formula"
)

// Cell is the atomic unit of a sheet. Raw is the source of truth (a literal
// value or a formula string like "=SUM(A1:A10)"). Value/ValueType are an
// optional client-reported cache of the computed result; the backend core
// never computes them. StyleId references the workbook StylePool.
type Cell struct {
	Raw       string `json:"raw"`
	Value     string `json:"value,omitempty"`
	ValueType string `json:"valueType,omitempty"`
	StyleId   int    `json:"styleId"`
}

// Kind reports whether the raw content is a formula (leading '=') or a value.
func (c Cell) Kind() CellKind {
	if len(c.Raw) > 0 && c.Raw[0] == '=' {
		return KindFormula
	}
	return KindValue
}

// IsEmpty reports whether the cell carries no content and default styling,
// i.e. it can be dropped from sparse storage.
func (c Cell) IsEmpty() bool {
	return c.Raw == "" && c.StyleId == 0 && c.Value == ""
}
