package changeset

import (
	"errors"
	"fmt"
	"strings"
)

// TextLinesMutator is a class to iterate and modify texts which have several lines.
// It is used for applying Changesets on arrays of lines.
//
// Mutation operations have the same constraints as exports operations with respect to newlines,
// but not the other additional constraints (i.e. ins/del ordering, forbidden no-ops,
// non-mergeability, final newline). Can be used to mutate lists of strings where the last char
// of each string is not actually a newline, but for the purposes of N and L values, the caller
// should pretend it is, and for things to work right in that case, the input to the Insert
// method should be a single line with no newlines.
type TextLinesMutator struct {
	lines     *[]string
	curSplice []interface{} // [startIndex int, deleteCount int, ...lines string]
	inSplice  bool
	curLine   int
	curCol    int
}

// NewTextLinesMutator creates a new TextLinesMutator with the given lines.
// Lines are mutated in place.
func NewTextLinesMutator(lines *[]string) *TextLinesMutator {
	return &TextLinesMutator{
		lines:     lines,
		curSplice: []interface{}{0, 0},
		inSplice:  false,
		curLine:   0,
		curCol:    0,
	}
}

// linesGet gets a line from lines at the given index.
func (m *TextLinesMutator) linesGet(idx int) string {
	if idx >= 0 && idx < len(*m.lines) {
		return (*m.lines)[idx]
	}
	return ""
}

// linesSlice returns a slice from lines.
func (m *TextLinesMutator) linesSlice(start, end int) []string {
	if start < 0 {
		start = 0
	}
	if end > len(*m.lines) {
		end = len(*m.lines)
	}
	if start >= end {
		return []string{}
	}
	return (*m.lines)[start:end]
}

// linesLength returns the length of lines.
func (m *TextLinesMutator) linesLength() int {
	return len(*m.lines)
}

// enterSplice starts a new splice.
func (m *TextLinesMutator) enterSplice() {
	m.curSplice[0] = m.curLine
	m.curSplice[1] = 0
	if m.curCol > 0 {
		m.putCurLineInSplice()
	}
	m.inSplice = true
}

func (m *TextLinesMutator) leaveSplice() {
	startIdx := m.curSplice[0].(int)
	deleteCount := m.curSplice[1].(int)

	// Build the new lines to insert
	var newLines []string
	for i := 2; i < len(m.curSplice); i++ {
		newLines = append(newLines, m.curSplice[i].(string))
	}

	// Perform the splice operation
	// Remove deleteCount lines starting at startIdx, and insert newLines
	endIdx := startIdx + deleteCount
	if endIdx > len(*m.lines) {
		endIdx = len(*m.lines)
	}

	result := make([]string, 0, len(*m.lines)-deleteCount+len(newLines))
	result = append(result, (*m.lines)[:startIdx]...)
	result = append(result, newLines...)
	result = append(result, (*m.lines)[endIdx:]...)

	*m.lines = result

	m.curSplice = []interface{}{0, 0}
	m.inSplice = false
}

// isCurLineInSplice indicates if curLine is already in the splice.
func (m *TextLinesMutator) isCurLineInSplice() bool {
	startIdx := m.curSplice[0].(int)
	return m.curLine-startIdx < len(m.curSplice)-2
}

// putCurLineInSplice incorporates current line into the splice and marks its old position to be deleted.
// Returns the index of the added line in curSplice.
func (m *TextLinesMutator) putCurLineInSplice() int {
	if !m.isCurLineInSplice() {
		startIdx := m.curSplice[0].(int)
		deleteCount := m.curSplice[1].(int)
		m.curSplice = append(m.curSplice, m.linesGet(startIdx+deleteCount))
		m.curSplice[1] = deleteCount + 1
	}
	startIdx := m.curSplice[0].(int)
	return 2 + m.curLine - startIdx
}

// SkipLines skips some newlines by putting them into the splice.
func (m *TextLinesMutator) SkipLines(L int, includeInSplice bool) {
	if L == 0 {
		return
	}

	if includeInSplice {
		if !m.inSplice {
			m.enterSplice()
		}
		for i := 0; i < L; i++ {
			m.curCol = 0
			m.putCurLineInSplice()
			m.curLine++
		}
	} else {
		if m.inSplice {
			if L > 1 {
				m.leaveSplice()
			} else {
				m.putCurLineInSplice()
			}
		}
		m.curLine += L
		m.curCol = 0
	}
}

// Skip skips some characters. Can contain newlines.
func (m *TextLinesMutator) Skip(N, L int, includeInSplice bool) {
	if N == 0 {
		return
	}

	if L > 0 {
		m.SkipLines(L, includeInSplice)
	} else {
		if includeInSplice && !m.inSplice {
			m.enterSplice()
		}
		if m.inSplice {
			m.putCurLineInSplice()
		}
		m.curCol += N
	}
}

// RemoveLines removes whole lines from lines array.
func (m *TextLinesMutator) RemoveLines(L int) string {
	if L == 0 {
		return ""
	}

	if !m.inSplice {
		m.enterSplice()
	}

	// nextKLinesText gets a string of joined lines after the end of the splice.
	nextKLinesText := func(k int) string {
		startIdx := m.curSplice[0].(int)
		deleteCount := m.curSplice[1].(int)
		start := startIdx + deleteCount
		end := start + k
		return strings.Join(m.linesSlice(start, end), "")
	}

	removed := ""
	if m.isCurLineInSplice() {
		if m.curCol == 0 {
			slineIdx := len(m.curSplice) - 1
			removed = m.curSplice[slineIdx].(string)
			m.curSplice = m.curSplice[:slineIdx]
			removed += nextKLinesText(L - 1)
			deleteCount := m.curSplice[1].(int)
			m.curSplice[1] = deleteCount + L - 1
		} else {
			removed = nextKLinesText(L - 1)
			deleteCount := m.curSplice[1].(int)
			m.curSplice[1] = deleteCount + L - 1
			sline := len(m.curSplice) - 1
			slineStr := m.curSplice[sline].(string)
			removed = slineStr[m.curCol:] + removed
			startIdx := m.curSplice[0].(int)
			deleteCount = m.curSplice[1].(int)
			m.curSplice[sline] = slineStr[:m.curCol] + m.linesGet(startIdx+deleteCount)
			m.curSplice[1] = deleteCount + 1
		}
	} else {
		removed = nextKLinesText(L)
		deleteCount := m.curSplice[1].(int)
		m.curSplice[1] = deleteCount + L
	}

	return removed
}

// Remove removes text from lines array.
func (m *TextLinesMutator) Remove(N, L int) string {
	if N == 0 {
		return ""
	}

	if L > 0 {
		return m.RemoveLines(L)
	}

	if !m.inSplice {
		m.enterSplice()
	}

	sline := m.putCurLineInSplice()
	slineStr := m.curSplice[sline].(string)

	endCol := m.curCol + N
	if endCol > len(slineStr) {
		endCol = len(slineStr)
	}

	removed := slineStr[m.curCol:endCol]
	m.curSplice[sline] = slineStr[:m.curCol] + slineStr[endCol:]

	return removed
}

// Insert inserts text into lines array.
func (m *TextLinesMutator) Insert(text string, L int) error {
	if text == "" {
		return nil
	}

	if !m.inSplice {
		m.enterSplice()
	}

	if L > 0 {
		newLines := SplitTextLines(text)

		if m.isCurLineInSplice() {
			sline := len(m.curSplice) - 1
			theLine := m.curSplice[sline].(string)
			lineCol := m.curCol

			// Insert the chars up to curCol and the first new line
			m.curSplice[sline] = theLine[:lineCol] + newLines[0]
			m.curLine++
			newLines = newLines[1:]

			// Insert the remaining new lines
			for _, line := range newLines {
				m.curSplice = append(m.curSplice, line)
			}
			m.curLine += len(newLines)

			// Insert the remaining chars from the "old" line
			m.curSplice = append(m.curSplice, theLine[lineCol:])
			m.curCol = 0
		} else {
			for _, line := range newLines {
				m.curSplice = append(m.curSplice, line)
			}
			m.curLine += len(newLines)
		}
	} else {
		// There are no additional lines
		sline := m.putCurLineInSplice()

		if sline >= len(m.curSplice) {
			return errors.New(fmt.Sprintf(
				"curSplice[sline] not populated, actual curSplice length is %d. "+
					"Possibly related to issues with splice operations",
				len(m.curSplice)))
		}

		slineStr := m.curSplice[sline].(string)
		m.curSplice[sline] = slineStr[:m.curCol] + text + slineStr[m.curCol:]
		m.curCol += len(text)
	}

	return nil
}

// HasMore checks if curLine (the line we are in when curSplice is applied) is the last line in lines.
// Returns true if there are lines left.
func (m *TextLinesMutator) HasMore() bool {
	docLines := m.linesLength()
	if m.inSplice {
		deleteCount := m.curSplice[1].(int)
		docLines += len(m.curSplice) - 2 - deleteCount
	}
	return m.curLine < docLines
}

// Close closes the splice.
func (m *TextLinesMutator) Close() {
	if m.inSplice {
		m.leaveSplice()
	}
}

// GetLines returns the current state of the lines.
func (m *TextLinesMutator) GetLines() []string {
	return *m.lines
}
