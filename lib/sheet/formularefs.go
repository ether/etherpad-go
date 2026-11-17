package sheet

import (
	"strconv"
	"strings"
	"unicode"
)

// Excel's grid limits. A reference shifted past them becomes #REF!.
const (
	maxSheetRows = 1048576
	maxSheetCols = 16384
)

// RefEdit describes how an op changes the cell references in formulas:
// inserting or deleting rows/columns moves them, renaming a sheet renames
// them, deleting a sheet breaks them (#REF!), as in Excel.
//
// formularefs.ts ports this exactly; both read testdata/formularefs.json.
type RefEdit struct {
	// Target is the name of the sheet the op changes.
	Target string
	// OnTarget: the formula is on that sheet, so references without a sheet
	// name point into it.
	OnTarget bool
	// Axis is "row" or "col" for an insert/delete; Delta > 0 inserts Delta
	// rows/columns at Index, Delta < 0 deletes -Delta of them from Index on.
	Axis  string
	Index int
	Delta int
	// Rename is the sheet's new name (renameSheet).
	Rename string
	// Deleted: the sheet is deleted (deleteSheet).
	Deleted bool
}

// refEnd is one end of a reference: a cell (col and row), a whole column
// (row < 0) or a whole row (col < 0).
type refEnd struct {
	col, row       int
	absCol, absRow bool
}

func (e refEnd) kind() int {
	switch {
	case e.col >= 0 && e.row >= 0:
		return 0 // cell
	case e.row < 0:
		return 1 // column
	default:
		return 2 // row
	}
}

// RewriteFormula returns raw with its references changed by e. Text that is
// not a reference (strings, function names, numbers) is kept as written, as
// are references the edit does not touch.
func RewriteFormula(raw string, e RefEdit) string {
	if !strings.HasPrefix(raw, "=") {
		return raw
	}
	rs := []rune(raw)
	var out strings.Builder
	out.Grow(len(raw))
	changed := false
	i := 0
	for i < len(rs) {
		c := rs[i]
		if c == '"' {
			j := i + 1
			for j < len(rs) {
				if rs[j] == '"' {
					if j+1 < len(rs) && rs[j+1] == '"' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			out.WriteString(string(rs[i:j]))
			i = j
			continue
		}
		if c == '\'' || c == '$' || isRefIdentChar(c) {
			if end, sheet, hasSheet, prefixEnd, a, b, hasB, ok := parseRef(rs, i); ok {
				orig := string(rs[i:end])
				repl := applyRefEdit(e, sheet, hasSheet, string(rs[i:prefixEnd]), a, b, hasB, orig)
				if repl != orig {
					changed = true
				}
				out.WriteString(repl)
				i = end
				continue
			}
			// Not a reference: copy the whole word, so the next try starts at
			// a word boundary (no "LOG10" -> "G10").
			j := i + 1
			if isRefIdentChar(c) {
				for j < len(rs) && isRefIdentChar(rs[j]) {
					j++
				}
			}
			out.WriteString(string(rs[i:j]))
			i = j
			continue
		}
		out.WriteRune(c)
		i++
	}
	if !changed {
		return raw
	}
	return out.String()
}

func isRefIdentChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '.'
}

func isASCIILetter(c rune) bool { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') }
func isASCIIDigit(c rune) bool  { return c >= '0' && c <= '9' }

// parseRef reads a reference at rs[i:]: an optional sheet name (Name! or
// 'Quoted Name'!), then A1, $A$1, A1:B2, A:C or 1:3.
func parseRef(rs []rune, i int) (end int, sheet string, hasSheet bool, prefixEnd int, a, b refEnd, hasB, ok bool) {
	p := i
	if rs[p] == '\'' {
		var name strings.Builder
		j := p + 1
		closed := false
		for j < len(rs) {
			if rs[j] == '\'' {
				if j+1 < len(rs) && rs[j+1] == '\'' {
					name.WriteRune('\'')
					j += 2
					continue
				}
				closed = true
				j++
				break
			}
			name.WriteRune(rs[j])
			j++
		}
		if !closed || j >= len(rs) || rs[j] != '!' {
			return
		}
		sheet, hasSheet, p = name.String(), true, j+1
	} else if isRefIdentChar(rs[p]) {
		j := p
		for j < len(rs) && isRefIdentChar(rs[j]) {
			j++
		}
		if j < len(rs) && rs[j] == '!' {
			sheet, hasSheet, p = string(rs[p:j]), true, j+1
		}
	}
	prefixEnd = p
	var aok bool
	a, p, aok = parseRefEnd(rs, p)
	if !aok {
		return
	}
	if p < len(rs) && rs[p] == ':' {
		if bb, q, bok := parseRefEnd(rs, p+1); bok && bb.kind() == a.kind() {
			b, hasB, p = bb, true, q
		}
	}
	if !hasB && a.kind() != 0 {
		return // a lone column or row is a name, not a reference
	}
	if p < len(rs) && (isRefIdentChar(rs[p]) || rs[p] == '(' || rs[p] == '!' || rs[p] == '$') {
		return // part of a longer name or a function call (LOG10(...))
	}
	return p, sheet, hasSheet, prefixEnd, a, b, hasB, true
}

func parseRefEnd(rs []rune, p int) (refEnd, int, bool) {
	e := refEnd{col: -1, row: -1}
	if p < len(rs) && rs[p] == '$' {
		e.absCol = true
		p++
	}
	j := p
	col := 0
	for j < len(rs) && isASCIILetter(rs[j]) && j-p < 4 {
		col = col*26 + int(unicode.ToUpper(rs[j])-'A'+1)
		j++
	}
	letters := j - p
	if letters > 3 {
		return e, p, false
	}
	if letters > 0 {
		e.col = col - 1
		p = j
		if p < len(rs) && rs[p] == '$' {
			e.absRow = true
			p++
		}
	} else if e.absCol {
		// "$1": a row reference written absolute
		e.absCol, e.absRow = false, true
	}
	j = p
	for j < len(rs) && isASCIIDigit(rs[j]) && j-p < 8 {
		j++
	}
	if j > p {
		n, err := strconv.Atoi(string(rs[p:j]))
		if err != nil || n < 1 {
			return e, p, false
		}
		e.row = n - 1
		p = j
	} else if e.absRow && letters > 0 {
		return e, p, false // "A$" without a row
	}
	if e.col < 0 && e.row < 0 {
		return e, p, false
	}
	if e.col >= maxSheetCols || e.row >= maxSheetRows {
		return e, p, false
	}
	return e, p, true
}

func sameSheetName(a, b string) bool { return strings.ToLower(a) == strings.ToLower(b) }

func applyRefEdit(e RefEdit, sheet string, hasSheet bool, prefix string, a, b refEnd, hasB bool, orig string) string {
	if hasSheet {
		if !sameSheetName(sheet, e.Target) {
			return orig
		}
	} else if !e.OnTarget {
		return orig
	}
	if e.Deleted {
		if !hasSheet {
			return orig
		}
		return "#REF!"
	}
	if e.Rename != "" {
		if !hasSheet {
			return orig
		}
		return QuoteSheetName(e.Rename) + "!" + orig[len(prefix):]
	}
	if e.Axis != "row" && e.Axis != "col" || e.Delta == 0 {
		return orig
	}
	get := func(x refEnd) int {
		if e.Axis == "row" {
			return x.row
		}
		return x.col
	}
	set := func(x *refEnd, v int) {
		if e.Axis == "row" {
			x.row = v
		} else {
			x.col = v
		}
	}
	limit := maxSheetCols
	if e.Axis == "row" {
		limit = maxSheetRows
	}
	if get(a) < 0 {
		return orig // a whole column under a row edit, or the other way round
	}
	a0, b0 := a, b
	if !hasB {
		v, ok := shiftRefCoord(get(a), e.Index, e.Delta)
		if !ok || v >= limit {
			return "#REF!"
		}
		set(&a, v)
	} else {
		lo, hi := get(a), get(b)
		swapped := lo > hi
		if swapped {
			lo, hi = hi, lo
		}
		nlo, nhi := shiftRangeCoords(lo, hi, e.Index, e.Delta)
		if nlo > nhi || nhi >= limit {
			return "#REF!"
		}
		if swapped {
			nlo, nhi = nhi, nlo
		}
		set(&a, nlo)
		set(&b, nhi)
	}
	if a == a0 && b == b0 {
		return orig // not moved: keep it as written
	}
	s := prefix + formatRefEnd(a)
	if hasB {
		s += ":" + formatRefEnd(b)
	}
	return s
}

// shiftRefCoord moves one coordinate; false when it lies in a deleted band.
func shiftRefCoord(v, index, delta int) (int, bool) {
	if delta > 0 {
		if v >= index {
			return v + delta, true
		}
		return v, true
	}
	band := -delta
	switch {
	case v < index:
		return v, true
	case v < index+band:
		return 0, false
	default:
		return v - band, true
	}
}

// shiftRangeCoords moves both ends of a range: an insert inside it grows it,
// a delete shrinks it. lo > hi in the result means it was deleted entirely.
func shiftRangeCoords(lo, hi, index, delta int) (int, int) {
	if delta > 0 {
		if lo >= index {
			lo += delta
		}
		if hi >= index {
			hi += delta
		}
		return lo, hi
	}
	band := -delta
	switch {
	case lo >= index+band:
		lo -= band
	case lo >= index:
		lo = index
	}
	switch {
	case hi >= index+band:
		hi -= band
	case hi >= index:
		hi = index - 1
	}
	return lo, hi
}

func formatRefEnd(e refEnd) string {
	var sb strings.Builder
	if e.col >= 0 {
		if e.absCol {
			sb.WriteByte('$')
		}
		sb.WriteString(ColumnName(e.col))
	}
	if e.row >= 0 {
		if e.absRow {
			sb.WriteByte('$')
		}
		sb.WriteString(strconv.Itoa(e.row + 1))
	}
	return sb.String()
}

// ColumnName turns a zero-based column index into its letters (0 -> A).
func ColumnName(col int) string {
	var b []byte
	for n := col + 1; n > 0; n = (n - 1) / 26 {
		b = append([]byte{byte('A' + (n-1)%26)}, b...)
	}
	return string(b)
}

// QuoteSheetName writes a sheet name the way a formula needs it: plain when
// it is a simple word, otherwise in single quotes ('My Sheet', 'Q1”24').
func QuoteSheetName(name string) string {
	plain := name != ""
	for i, c := range name {
		if i == 0 && !(unicode.IsLetter(c) || c == '_') || !isRefIdentChar(c) {
			plain = false
			break
		}
	}
	if plain {
		// Names that read as a reference or a boolean need quotes too.
		if _, _, _, _, _, _, _, ok := parseRef([]rune(name), 0); ok {
			plain = false
		}
		if u := strings.ToUpper(name); u == "TRUE" || u == "FALSE" {
			plain = false
		}
	}
	if plain {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// rewriteRefs changes the formulas of every sheet by e. target is the sheet
// the op changes (nil once it is gone); its own formulas see references
// without a sheet name as pointing into it.
func (w *Workbook) rewriteRefs(target *Sheet, e RefEdit) {
	for _, s := range w.Sheets {
		e.OnTarget = s == target
		for ref, c := range s.Cells {
			if c.Kind() != KindFormula {
				continue
			}
			if n := RewriteFormula(c.Raw, e); n != c.Raw {
				c.Raw, c.Value, c.ValueType = n, "", ""
				s.Cells[ref] = c
			}
		}
	}
}
