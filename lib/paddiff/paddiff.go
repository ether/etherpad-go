// Package paddiff is a port of the original Etherpad src/node/utils/padDiff.ts.
//
// It composes the changesets between two revisions of a pad into a single
// "diff" atext: all insertions of the range keep their author attribution and
// all deletions are re-inserted at the position they were deleted from,
// carrying a 'removed' attribute plus the author who deleted them. The
// resulting atext can be rendered with the regular export-HTML pipeline
// (lib/io/exportHtml.go understands the 'removed' attribute) to visualize the
// changes between the two revisions.
package paddiff

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/utils"
)

// Pad is the subset of *padModel.Pad that is needed to build a diff. It is an
// interface so the diff logic can be unit tested with an in-memory fake.
type Pad interface {
	GetInternalRevisionAText(targetRev int) *apool.AText
	GetRevision(revNumber int) (*db2.PadSingleRevision, error)
}

// GetValidRevisionRange mirrors Pad.getValidRevisionRange of the original
// Etherpad: startRev must lie within [0, head]; endRev defaults to head when
// nil, is clamped to head and must not be lower than startRev. ok is false
// when the range is invalid.
func GetValidRevisionRange(startRev int, endRev *int, head int) (from int, to int, ok bool) {
	if startRev < 0 || startRev > head {
		return 0, 0, false
	}
	end := head
	if endRev != nil {
		end = *endRev
	}
	if end < startRev {
		return 0, 0, false
	}
	if end > head {
		end = head
	}
	return startRev, end, true
}

// CreateDiffAText builds the diff atext between fromRev and toRev (both
// inclusive endpoints of the revision range, fromRev <= toRev <= head) and
// returns it together with the list of authors that contributed changes in
// that range. Like the original PadDiff, it adds the needed 'author' and
// 'removed' attributes to the supplied pool (in memory only; the pad record
// itself is not saved).
func CreateDiffAText(p Pad, pool *apool.APool, fromRev int, toRev int) (*apool.AText, []string, error) {
	startAText := p.GetInternalRevisionAText(fromRev)
	if startAText == nil {
		return nil, nil, fmt.Errorf("could not load atext of revision %d", fromRev)
	}

	// Strip the authorship of the start atext so that only the changes of the
	// requested range are attributed (original: _createClearStartAtext).
	atext, err := createClearStartAText(*startAText, pool)
	if err != nil {
		return nil, nil, err
	}

	authors := make([]string, 0)
	var superChangeset *string

	for rev := fromRev + 1; rev <= toRev; rev++ {
		revision, err := p.GetRevision(rev)
		if err != nil {
			return nil, nil, err
		}

		// Skip clearAuthorship changesets — they would wipe the authorship
		// attribution we are trying to display.
		if isClearAuthorship(revision.Changeset, pool) {
			continue
		}

		author := ""
		if revision.AuthorId != nil {
			author = *revision.AuthorId
		}

		cs, err := extendChangesetWithAuthor(revision.Changeset, author, pool)
		if err != nil {
			return nil, nil, err
		}

		if !contains(authors, author) {
			authors = append(authors, author)
		}

		if superChangeset == nil {
			superChangeset = &cs
		} else {
			composed, err := changeset.Compose(*superChangeset, cs, pool)
			if err != nil {
				return nil, nil, err
			}
			superChangeset = composed
		}
	}

	// If there are only clearAuthorship changesets we don't get a
	// superChangeset, so we can skip this step.
	if superChangeset != nil {
		deletionChangeset, err := createDeletionChangeset(*superChangeset, atext, pool)
		if err != nil {
			return nil, nil, err
		}

		// Apply the superChangeset, which includes all the insertions.
		newAText, err := changeset.ApplyToAText(*superChangeset, atext, *pool)
		if err != nil {
			return nil, nil, err
		}
		atext = *newAText

		// Apply the deletionChangeset, which re-adds the deletions.
		newAText, err = changeset.ApplyToAText(deletionChangeset, atext, *pool)
		if err != nil {
			return nil, nil, err
		}
		atext = *newAText
	}

	return &atext, authors, nil
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// createClearAuthorship builds a changeset that keeps the whole text while
// setting the 'author' attribute to the empty string (original:
// _createClearAuthorship).
func createClearAuthorship(atext apool.AText, pool *apool.APool) string {
	authorAttrib := pool.PutAttrib(apool.Attribute{Key: "author", Value: ""}, nil)
	attribs := "*" + utils.NumToString(authorAttrib)

	builder := newOpBuilder(utf8.RuneCountInString(atext.Text))
	builder.keepText(atext.Text, attribs)
	return builder.toString()
}

// createClearStartAText returns the atext with all authorship cleared
// (original: _createClearStartAtext).
func createClearStartAText(atext apool.AText, pool *apool.APool) (apool.AText, error) {
	cs := createClearAuthorship(atext, pool)
	newAText, err := changeset.ApplyToAText(cs, atext, *pool)
	if err != nil {
		return atext, err
	}
	return *newAText, nil
}

// isClearAuthorship checks whether the changeset only resets the author
// attribute of the whole text to the anonymous author (original:
// _isClearAuthorship).
func isClearAuthorship(cs string, pool *apool.APool) bool {
	unpacked, err := changeset.Unpack(cs)
	if err != nil {
		return false
	}

	// check if there is nothing in the charBank and oldLength == newLength
	if unpacked.CharBank != "" || unpacked.OldLen != unpacked.NewLen {
		return false
	}

	ops, err := changeset.DeserializeOps(unpacked.Ops)
	if err != nil || ops == nil || len(*ops) != 1 {
		return false
	}
	clearOperator := (*ops)[0]

	// check if this operator doesn't change text
	if clearOperator.OpCode != "=" {
		return false
	}

	// check that this operator applies to the complete text. If the text ends
	// with a new line, it is exactly one character less, else it has the same
	// length.
	if clearOperator.Chars != unpacked.OldLen-1 && clearOperator.Chars != unpacked.OldLen {
		return false
	}

	// Check that the operation has exactly one attribute and that it is an
	// anonymous author attribute.
	appliedAttribs := changeset.AttribsFromString(clearOperator.Attribs, *pool)
	if len(appliedAttribs) != 1 {
		return false
	}
	return appliedAttribs[0].Key == "author" && appliedAttribs[0].Value == ""
}

// extendChangesetWithAuthor marks all deletions of the changeset with the
// author who performed them plus a 'removed' attribute, and attribute-only
// changes with the author (original: _extendChangesetWithAuthor).
func extendChangesetWithAuthor(cs string, author string, pool *apool.APool) (string, error) {
	unpacked, err := changeset.Unpack(cs)
	if err != nil {
		return "", err
	}
	ops, err := changeset.DeserializeOps(unpacked.Ops)
	if err != nil {
		return "", err
	}

	assem := changeset.NewOpAssembler()

	authorAttrib := pool.PutAttrib(apool.Attribute{Key: "author", Value: author}, nil)
	deletedAttrib := pool.PutAttrib(apool.Attribute{Key: "removed", Value: "true"}, nil)
	attribs := "*" + utils.NumToString(authorAttrib) + "*" + utils.NumToString(deletedAttrib)

	for _, operator := range *ops {
		if operator.OpCode == "-" {
			// this is a delete operator, extend it with the author
			operator.Attribs = attribs
		} else if operator.OpCode == "=" && operator.Attribs != "" {
			// this operator changes only attributes, mark which author did that
			operator.Attribs += "*" + utils.NumToString(authorAttrib)
		}
		assem.Append(operator)
	}

	return changeset.Pack(unpacked.OldLen, unpacked.NewLen, assem.String(), unpacked.CharBank), nil
}

// createDeletionChangeset builds a changeset (applying to the result of cs)
// that re-inserts all text deleted by cs, carrying the attributes the text had
// before the deletion plus the 'removed'/author attributes that
// extendChangesetWithAuthor attached to the delete ops (original:
// _createDeletionChangeset).
func createDeletionChangeset(cs string, startAText apool.AText, pool *apool.APool) (string, error) {
	lines := changeset.SplitTextLines(startAText.Text)
	alines, err := changeset.SplitAttributionLines(startAText.Attribs, startAText.Text)
	if err != nil {
		return "", err
	}

	// lines and alines are what the changeset is meant to apply to. They
	// include final newlines on lines.
	linesGet := func(idx int) string {
		if idx >= 0 && idx < len(lines) {
			return lines[idx]
		}
		return ""
	}
	aLinesGet := func(idx int) string {
		if idx >= 0 && idx < len(alines) {
			return alines[idx]
		}
		return ""
	}

	curLine := 0
	curChar := 0
	curLineOpsLoaded := false
	var curLineOps []changeset.Op
	curLineOpsIdx := 0
	curLineOpsLine := 0
	plus := "+"
	curLineNextOp := changeset.NewOp(&plus)
	// mirrors the `curLineOpsNext = curLineOps.next()` generator state of the
	// original implementation
	curLineOpsNextDone := true
	var curLineOpsNextVal changeset.Op

	loadLineOps := func(lineIdx int) error {
		ops, err := changeset.DeserializeOps(aLinesGet(lineIdx))
		if err != nil {
			return err
		}
		if ops == nil {
			curLineOps = []changeset.Op{}
		} else {
			curLineOps = *ops
		}
		curLineOpsIdx = 0
		curLineOpsLoaded = true
		return nil
	}
	nextLineOp := func() {
		if curLineOpsIdx < len(curLineOps) {
			curLineOpsNextVal = curLineOps[curLineOpsIdx]
			curLineOpsIdx++
			curLineOpsNextDone = false
		} else {
			curLineOpsNextVal = changeset.NewOp(nil)
			curLineOpsNextDone = true
		}
	}

	unpacked, err := changeset.Unpack(cs)
	if err != nil {
		return "", err
	}
	builder := newOpBuilder(unpacked.NewLen)

	consumeAttribRuns := func(numChars int, f func(n int, attribs string, endsLine bool)) error {
		if !curLineOpsLoaded || curLineOpsLine != curLine {
			if err := loadLineOps(curLine); err != nil {
				return err
			}
			nextLineOp()
			curLineOpsLine = curLine
			indexIntoLine := 0
			for !curLineOpsNextDone {
				curLineNextOp = curLineOpsNextVal
				nextLineOp()
				if indexIntoLine+curLineNextOp.Chars >= curChar {
					curLineNextOp.Chars -= curChar - indexIntoLine
					break
				}
				indexIntoLine += curLineNextOp.Chars
			}
		}

		for numChars > 0 {
			if curLineNextOp.Chars == 0 && curLineOpsNextDone {
				curLine++
				curChar = 0
				curLineOpsLine = curLine
				curLineNextOp.Chars = 0
				if err := loadLineOps(curLine); err != nil {
					return err
				}
				nextLineOp()
			}

			if curLineNextOp.Chars == 0 {
				if curLineOpsNextDone {
					curLineNextOp = changeset.NewOp(nil)
				} else {
					curLineNextOp = curLineOpsNextVal
					nextLineOp()
				}
			}

			if curLineNextOp.Chars == 0 {
				// Defensive: the original relies on well-formed attribution
				// lines; bail out instead of looping forever on malformed input.
				return errors.New("ran out of attribution ops while consuming attrib runs")
			}

			charsToUse := numChars
			if curLineNextOp.Chars < charsToUse {
				charsToUse = curLineNextOp.Chars
			}

			f(charsToUse, curLineNextOp.Attribs,
				charsToUse == curLineNextOp.Chars && curLineNextOp.Lines > 0)
			numChars -= charsToUse
			curLineNextOp.Chars -= charsToUse
			curChar += charsToUse
		}

		if curLineNextOp.Chars == 0 && curLineOpsNextDone {
			curLine++
			curChar = 0
		}
		return nil
	}

	skip := func(n int, l int) error {
		if l > 0 {
			curLine += l
			curChar = 0
		} else if curLineOpsLoaded && curLineOpsLine == curLine {
			return consumeAttribRuns(n, func(int, string, bool) {})
		} else {
			curChar += n
		}
		return nil
	}

	nextText := func(numChars int) string {
		collected := make([]rune, 0, numChars)
		firstRunes := []rune(linesGet(curLine))
		if curChar < len(firstRunes) {
			collected = append(collected, firstRunes[curChar:]...)
		}

		lineNum := curLine + 1
		for len(collected) < numChars && lineNum <= len(lines) {
			collected = append(collected, []rune(linesGet(lineNum))...)
			lineNum++
		}

		if len(collected) > numChars {
			collected = collected[:numChars]
		}
		return string(collected)
	}

	csOps, err := changeset.DeserializeOps(unpacked.Ops)
	if err != nil {
		return "", err
	}

	for _, csOp := range *csOps {
		switch csOp.OpCode {
		case "=":
			textBank := nextText(csOp.Chars)

			// Decide whether this equal operator is an attribute change. If
			// the text this operator applies to is only a star, then this is a
			// false positive and should be ignored.
			if csOp.Attribs != "" && textBank != "*" {
				attribs := changeset.FromString(csOp.Attribs, pool)
				undoCache := make(map[string]string)
				undoBackToAttribs := func(oldAttribsStr string) string {
					if cached, ok := undoCache[oldAttribsStr]; ok {
						return cached
					}
					oldAttribs := changeset.FromString(oldAttribsStr, pool)
					backAttribs := changeset.NewAttributeMap(pool).
						Set("author", "").
						Set("removed", "true")
					for key, value := range attribs.Iter() {
						oldValue := ""
						if v := oldAttribs.Get(key); v != nil {
							oldValue = *v
						}
						if oldValue != value {
							backAttribs.Set(key, oldValue)
						}
					}
					result := backAttribs.String()
					undoCache[oldAttribsStr] = result
					return result
				}

				textLeftToProcess := []rune(textBank)
				for len(textLeftToProcess) > 0 {
					// process till the next line break or process only one
					// line break
					lengthToProcess := indexOfRune(textLeftToProcess, '\n')
					lineBreak := false
					switch lengthToProcess {
					case -1:
						lengthToProcess = len(textLeftToProcess)
					case 0:
						lineBreak = true
						lengthToProcess = 1
					}

					processText := textLeftToProcess[:lengthToProcess]
					textLeftToProcess = textLeftToProcess[lengthToProcess:]

					if lineBreak {
						// just skip linebreaks, don't do an insert + keep for
						// a linebreak
						builder.keep(1, 1, "")

						// consume the attributes of this linebreak
						if err := consumeAttribRuns(1, func(int, string, bool) {}); err != nil {
							return "", err
						}
					} else {
						// add the old text via an insert, with a deletion
						// attribute + the author attribute of the author who
						// deleted it
						textBankIndex := 0
						if err := consumeAttribRuns(lengthToProcess, func(n int, attribs string, endsLine bool) {
							oldAttribs := undoBackToAttribs(attribs)
							builder.insert(string(processText[textBankIndex:textBankIndex+n]), oldAttribs)
							textBankIndex += n
						}); err != nil {
							return "", err
						}

						builder.keep(lengthToProcess, 0, "")
					}
				}
			} else {
				if err := skip(csOp.Chars, csOp.Lines); err != nil {
					return "", err
				}
				builder.keep(csOp.Chars, csOp.Lines, "")
			}
		case "+":
			builder.keep(csOp.Chars, csOp.Lines, "")
		case "-":
			textBank := []rune(nextText(csOp.Chars))
			textBankIndex := 0
			if err := consumeAttribRuns(csOp.Chars, func(n int, attribs string, endsLine bool) {
				builder.insert(string(textBank[textBankIndex:textBankIndex+n]), attribs+csOp.Attribs)
				textBankIndex += n
			}); err != nil {
				return "", err
			}
		}
	}

	result, err := changeset.CheckRep(builder.toString())
	if err != nil {
		return "", err
	}
	return *result, nil
}

func indexOfRune(runes []rune, r rune) int {
	for i, c := range runes {
		if c == r {
			return i
		}
	}
	return -1
}
