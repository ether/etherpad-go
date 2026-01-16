package models

// LineModel represents a parsed line with its attributes for export
type LineModel struct {
	ListLevel    int
	Text         []rune
	Aline        string
	ListTypeName string
	Start        string
}
