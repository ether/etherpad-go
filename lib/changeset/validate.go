package changeset

import (
	"fmt"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/utils"
)

// ValidateWellFormed walks a changeset and checks per-op invariants that must
// hold for the string to be safely composable with another changeset:
//
//   - every op has chars >= lines
//   - for '+' ops, the slice of the char bank that the op consumes contains
//     exactly op.Lines '\n' characters
//   - all char bank characters are consumed by '+' ops (no leftover, no underflow)
//
// It does NOT catch malformed '=' ops whose lines count disagrees with the
// underlying document text — that requires applying the changeset against a
// text buffer. Use ValidateRoundTrip for that class of bug.
//
// A non-nil error describes the first violation found.
func ValidateWellFormed(cs string) error {
	unpacked, err := Unpack(cs)
	if err != nil {
		return fmt.Errorf("unpack: %w", err)
	}
	ops, err := DeserializeOps(unpacked.Ops)
	if err != nil {
		return fmt.Errorf("deserialize ops: %w", err)
	}
	bankRunes := []rune(unpacked.CharBank)
	bankLen := len(bankRunes)
	bankPos := 0
	for i, op := range *ops {
		if op.Chars < op.Lines {
			return fmt.Errorf("op %d %q: chars (%d) < lines (%d)", i, op.String(), op.Chars, op.Lines)
		}
		if op.OpCode == "+" {
			if bankPos+op.Chars > bankLen {
				return fmt.Errorf("op %d %q: char bank underflow (need %d more, have %d)", i, op.String(), op.Chars, bankLen-bankPos)
			}
			nls := 0
			for _, r := range bankRunes[bankPos : bankPos+op.Chars] {
				if r == '\n' {
					nls++
				}
			}
			if nls != op.Lines {
				return fmt.Errorf("op %d %q: char bank slice has %d newline(s), op declares %d", i, op.String(), nls, op.Lines)
			}
			bankPos += op.Chars
		}
	}
	if bankPos != bankLen {
		return fmt.Errorf("char bank length mismatch: consumed %d, bank has %d", bankPos, bankLen)
	}
	return nil
}

// ValidateRoundTrip checks that applying `backward` to a copy of
// `postForward` yields `preForward`. This catches `=` ops whose declared
// line count disagrees with the document text — the exact class of bug that
// triggers the client-side "line count mismatch when composing changesets
// A*B" assertion. The caller's slices are not mutated.
func ValidateRoundTrip(backward string, postForward, preForward []string) error {
	result := make([]string, len(postForward))
	copy(result, postForward)
	if err := safeMutateTextLines(backward, &result); err != nil {
		return fmt.Errorf("apply backward: %w", err)
	}
	if len(result) != len(preForward) {
		return fmt.Errorf("round-trip line count mismatch: got %d, want %d", len(result), len(preForward))
	}
	for i := range preForward {
		if result[i] != preForward[i] {
			return fmt.Errorf("round-trip text mismatch at line %d: got %q, want %q",
				i, truncate(result[i], 120), truncate(preForward[i], 120))
		}
	}
	return nil
}

// safeMutateTextLines wraps MutateTextLines in a recover so that panics from
// malformed changesets surface as errors instead of crashing the caller.
func safeMutateTextLines(cs string, textLines *[]string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("MutateTextLines panicked: %v", r)
		}
	}()
	return MutateTextLines(cs, textLines)
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return utils.RuneSlice(s, 0, max) + "…"
}
