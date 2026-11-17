package xlsx

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Hyperlinks, notes, list validation and conditional formatting: per cell
// props in the model, per range objects in xlsx.

const noteAuthor = "Etherpad"

// sqrefOf writes cells as an xlsx sqref, joining runs of neighbouring cells
// in a row into ranges ("A1:C1 B3").
func sqrefOf(refs []sheet.CellRef) string {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Row != refs[j].Row {
			return refs[i].Row < refs[j].Row
		}
		return refs[i].Col < refs[j].Col
	})
	var parts []string
	for i := 0; i < len(refs); {
		j := i
		for j+1 < len(refs) && refs[j+1].Row == refs[i].Row && refs[j+1].Col == refs[j].Col+1 {
			j++
		}
		start, _ := excelize.CoordinatesToCellName(refs[i].Col+1, refs[i].Row+1)
		if j == i {
			parts = append(parts, start)
		} else {
			end, _ := excelize.CoordinatesToCellName(refs[j].Col+1, refs[j].Row+1)
			parts = append(parts, start+":"+end)
		}
		i = j + 1
	}
	return strings.Join(parts, " ")
}

// maxMetaCells bounds how many cells one import expands from the ranges of
// links, notes, validations and conditional formats together: a small file
// can repeat a large range thousands of times.
const maxMetaCells = 200000

// refBudget counts down the cells an import may still expand.
type refBudget struct{ left int }

// cellsOfSqref expands an sqref to the distinct cells inside the grid,
// stopping when the budget is spent.
func cellsOfSqref(sqref string, b *refBudget) []sheet.CellRef {
	var out []sheet.CellRef
	seen := map[sheet.CellRef]bool{}
	for _, part := range strings.Fields(strings.ReplaceAll(sqref, ",", " ")) {
		ends := strings.SplitN(part, ":", 2)
		c0, r0, err := excelize.CellNameToCoordinates(ends[0])
		if err != nil {
			continue
		}
		c1, r1 := c0, r0
		if len(ends) == 2 {
			if c1, r1, err = excelize.CellNameToCoordinates(ends[1]); err != nil {
				continue
			}
		}
		for r := r0; r <= min(r1, maxRows); r++ {
			for c := c0; c <= min(c1, maxCols); c++ {
				ref := sheet.CellRef{Row: r - 1, Col: c - 1}
				if seen[ref] {
					continue
				}
				if b.left <= 0 {
					return out
				}
				b.left--
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}
	return out
}

var condCriteria = map[string]string{
	"gt": ">", "lt": "<", "ge": ">=", "le": "<=", "eq": "==", "ne": "!=",
	"between": "between", "notBetween": "not between",
	"contains": "containing", "notContains": "not containing",
	"beginsWith": "begins with", "endsWith": "ends with",
}

// cellValue writes a rule value as an xlsx formula: numbers as they are,
// text in quotes.
func cellValue(v string) string {
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return v
	}
	return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
}

// valueOfCell reads cellValue back; false for formulas it cannot represent.
func valueOfCell(v string) (string, bool) {
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return v, true
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return strings.ReplaceAll(v[1:len(v)-1], `""`, `"`), true
	}
	return "", false
}

// exportCellMeta writes the meta props of one sheet.
func exportCellMeta(f *excelize.File, wb *sheet.Workbook, s *sheet.Sheet, name string) error {
	lists := map[string][]sheet.CellRef{}
	conds := map[string][]sheet.CellRef{}
	refs := make([]sheet.CellRef, 0, len(s.Cells))
	for ref := range s.Cells {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Row < refs[j].Row || refs[i].Row == refs[j].Row && refs[i].Col < refs[j].Col
	})
	for _, ref := range refs {
		cell := s.Cells[ref]
		if cell.StyleId == 0 {
			continue
		}
		st, ok := wb.Styles.Get(cell.StyleId)
		if !ok {
			continue
		}
		axis, err := excelize.CoordinatesToCellName(ref.Col+1, ref.Row+1)
		if err != nil {
			return err
		}
		if link := st.Props[sheet.PropLink]; link != "" {
			if err := f.SetCellHyperLink(name, axis, link, "External"); err != nil {
				return err
			}
		}
		if note := st.Props[sheet.PropNote]; note != "" {
			if err := f.AddComment(name, excelize.Comment{Cell: axis, Author: noteAuthor, Paragraph: []excelize.RichTextRun{{Text: note}}}); err != nil {
				return err
			}
		}
		if l := st.Props[sheet.PropValidList]; l != "" {
			lists[l] = append(lists[l], ref)
		}
		if cf := st.Props[sheet.PropCondFmt]; cf != "" {
			conds[cf] = append(conds[cf], ref)
		}
	}

	for _, l := range sortedKeys(lists) {
		dv := excelize.NewDataValidation(true)
		dv.SetSqref(sqrefOf(lists[l]))
		dv.Type = "list"
		dv.Formula1 = `"` + strings.ReplaceAll(l, `"`, `""`) + `"`
		dv.ShowDropDown = false // xlsx: false means the arrow IS shown
		dv.SetError(excelize.DataValidationErrorStyleStop, "", "")
		if err := f.AddDataValidation(name, dv); err != nil {
			return err
		}
	}

	for _, key := range sortedKeys(conds) {
		rules := sheet.ParseCondRules(key)
		var opts []excelize.ConditionalFormatOptions
		for _, r := range rules {
			style := &excelize.Style{}
			if r.Bg != "" {
				style.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{expandHex(r.Bg)}}
			}
			if r.Color != "" || r.Bold == "1" {
				style.Font = &excelize.Font{Bold: r.Bold == "1"}
				if r.Color != "" {
					style.Font.Color = expandHex(r.Color)
				}
			}
			id, err := f.NewConditionalStyle(style)
			if err != nil {
				return err
			}
			o := excelize.ConditionalFormatOptions{Format: &id}
			switch r.Op {
			case "blank":
				o.Type = "blanks"
			case "notBlank":
				o.Type = "no_blanks"
			case "contains", "notContains", "beginsWith", "endsWith":
				o.Type, o.Criteria, o.Value = "text", condCriteria[r.Op], r.V1
			case "between", "notBetween":
				o.Type, o.Criteria, o.MinValue, o.MaxValue = "cell", condCriteria[r.Op], cellValue(r.V1), cellValue(r.V2)
			default:
				o.Type, o.Criteria, o.Value = "cell", condCriteria[r.Op], cellValue(r.V1)
			}
			opts = append(opts, o)
		}
		if len(opts) == 0 {
			continue
		}
		if err := f.SetConditionalFormat(name, sqrefOf(conds[key]), opts); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// addProp sets one prop on a cell, keeping its other props.
func addProp(wb *sheet.Workbook, sh *sheet.Sheet, ref sheet.CellRef, key, value string) {
	cell := sh.GetCell(ref)
	props := map[string]string{}
	if st, ok := wb.Styles.Get(cell.StyleId); ok {
		for k, v := range st.Props {
			props[k] = v
		}
	}
	props[key] = value
	if sheet.ValidateProps(map[string]string{key: value}) != nil {
		return
	}
	cell.StyleId = wb.Styles.Put(sheet.Style{Props: props})
	sh.SetCell(ref, cell)
}

// importCellMeta reads what exportCellMeta writes, from any xlsx. Whatever
// the model cannot hold (links into the workbook, list sources that are
// ranges, other rule types) is skipped.
func importCellMeta(f *excelize.File, wb *sheet.Workbook, sh *sheet.Sheet, name string, b *refBudget, cfs []sheetCondFormat) {
	if cells, err := f.GetHyperLinkCells(name, "External"); err == nil {
		for _, axis := range cells {
			ok, target, err := f.GetCellHyperLink(name, axis)
			if err != nil || !ok {
				continue
			}
			for _, ref := range cellsOfSqref(axis, b) {
				addProp(wb, sh, ref, sheet.PropLink, target)
			}
		}
	}

	if comments, err := f.GetComments(name); err == nil {
		for _, cm := range comments {
			var text strings.Builder
			for _, run := range cm.Paragraph {
				text.WriteString(run.Text)
			}
			note := text.String()
			if note == "" {
				note = cm.Text
			}
			// Excel starts a note with "Author:\n"; drop it.
			if cm.Author != "" {
				note = strings.TrimPrefix(note, cm.Author+":\n")
				note = strings.TrimPrefix(note, cm.Author+":")
			}
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			if len(note) > sheet.MaxNoteLength {
				note = strings.ToValidUTF8(note[:sheet.MaxNoteLength], "")
			}
			for _, ref := range cellsOfSqref(cm.Cell, b) {
				addProp(wb, sh, ref, sheet.PropNote, note)
			}
		}
	}

	if dvs, err := f.GetDataValidations(name); err == nil {
		for _, dv := range dvs {
			if dv == nil || dv.Type != "list" {
				continue
			}
			list, ok := valueOfCell(dv.Formula1)
			if !ok || list == "" || len(list) > sheet.MaxValidListLength {
				continue
			}
			if _, err := strconv.ParseFloat(dv.Formula1, 64); err == nil {
				continue // a lone number is not a list
			}
			for _, ref := range cellsOfSqref(dv.Sqref, b) {
				addProp(wb, sh, ref, sheet.PropValidList, list)
			}
		}
	}

	// Conditional formats, in the file's rule priority: on screen the first
	// matching rule wins.
	type prioRule struct {
		prio, order int
		rule        sheet.CondRule
	}
	perCell := map[sheet.CellRef][]prioRule{}
	order := 0
	for _, cf := range cfs {
		for _, xr := range cf.Rules {
			r, ok := condRuleOfXML(f, xr)
			if !ok {
				continue
			}
			order++
			for _, ref := range cellsOfSqref(cf.Sqref, b) {
				perCell[ref] = append(perCell[ref], prioRule{xr.Priority, order, r})
			}
		}
	}
	for ref, list := range perCell {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].prio != list[j].prio {
				return list[i].prio < list[j].prio
			}
			return list[i].order < list[j].order
		})
		rules := make([]sheet.CondRule, 0, min(len(list), sheet.MaxCondRules))
		for _, pr := range list[:min(len(list), sheet.MaxCondRules)] {
			rules = append(rules, pr.rule)
		}
		if b, err := json.Marshal(rules); err == nil {
			addProp(wb, sh, ref, sheet.PropCondFmt, string(b))
		}
	}
}

var cellIsOps = map[string]string{
	"greaterThan": "gt", "lessThan": "lt", "greaterThanOrEqual": "ge", "lessThanOrEqual": "le",
	"equal": "eq", "notEqual": "ne", "between": "between", "notBetween": "notBetween",
}

var textRuleOps = map[string]string{
	"containsText": "contains", "notContainsText": "notContains", "beginsWith": "beginsWith", "endsWith": "endsWith",
}

// condRuleOfXML maps one cfRule of the sheet XML to a model rule; false for
// the kinds the model cannot hold (formulas, colour scales, top 10, ...).
func condRuleOfXML(f *excelize.File, x xmlCfRule) (sheet.CondRule, bool) {
	var r sheet.CondRule
	switch {
	case x.Type == "cellIs":
		r.Op = cellIsOps[x.Operator]
		if r.Op == "" || len(x.Formula) < sheet.CondOps[r.Op] {
			return r, false
		}
		v1, ok := valueOfCell(x.Formula[0])
		if !ok {
			return r, false
		}
		r.V1 = v1
		if sheet.CondOps[r.Op] == 2 {
			v2, ok := valueOfCell(x.Formula[1])
			if !ok {
				return r, false
			}
			r.V2 = v2
		}
	case textRuleOps[x.Type] != "":
		r.Op, r.V1 = textRuleOps[x.Type], x.Text
	case x.Type == "containsBlanks":
		r.Op = "blank"
	case x.Type == "notContainsBlanks":
		r.Op = "notBlank"
	default:
		return r, false
	}
	if x.DxfID == nil {
		return r, false
	}
	st, err := f.GetConditionalStyle(*x.DxfID)
	if err != nil || st == nil {
		return r, false
	}
	if st.Fill.Type == "pattern" && len(st.Fill.Color) > 0 {
		r.Bg = normalizeHex(st.Fill.Color[0])
	}
	if st.Font != nil {
		r.Color = normalizeHex(st.Font.Color)
		if st.Font.Bold {
			r.Bold = "1"
		}
	}
	if r.Bg == "" && r.Color == "" && r.Bold == "" {
		return r, false
	}
	b, _ := json.Marshal([]sheet.CondRule{r})
	if sheet.ParseCondRules(string(b)) == nil {
		return r, false
	}
	return r, true
}
