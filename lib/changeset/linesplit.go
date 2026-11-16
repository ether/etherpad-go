package changeset

import (
	"fmt"
	"strings"

	"github.com/ether/etherpad-go/lib/utils"
)

// SplitOpsAtLineBoundaries re-splits the keep (=) and delete (-) ops of cs
// at line boundaries, using text, the text cs applies to (etherpad-lite's
// splitOpsAtLineBoundaries, #8495).
//
// A multi-line keep or delete must end on a newline: line-based appliers
// such as MutateTextLines, which every browser uses for NEW_CHANGES, skip
// or remove whole lines and lose the characters of the partial line, while
// ApplyToText keeps them. CheckRep cannot see this, it does not know the
// text; clients have sent `|1=3` over "a\nb" where the canonical form is
// `|1=2=1` (etherpad-lite issue #8487). A changeset already in that form
// comes back unchanged.
func SplitOpsAtLineBoundaries(cs string, text string) (string, error) {
	unpacked, err := Unpack(cs)
	if err != nil {
		return "", err
	}
	if n := utils.RuneCount(text); n != unpacked.OldLen {
		return "", fmt.Errorf("mismatched apply: %d / %d", n, unpacked.OldLen)
	}
	ops, err := DeserializeOps(unpacked.Ops)
	if err != nil {
		return "", err
	}
	assem := NewSmartOpAssembler()
	pos := 0 // byte offset into text, moved along op by op
	for _, op := range *ops {
		if op.OpCode == "+" {
			assem.Append(op)
			continue
		}
		end := pos + utils.RuneByteOffset(text[pos:], op.Chars)
		slice := text[pos:end]
		pos = end
		if op.Lines == 0 || strings.HasSuffix(slice, "\n") {
			assem.Append(op)
			continue
		}
		attribs := op.Attribs
		for _, o := range OpsFromText(op.OpCode, slice, &KeepArgs{stringAttribs: &attribs}, nil) {
			assem.Append(o)
		}
	}
	assem.EndDocument()
	return Pack(unpacked.OldLen, unpacked.NewLen, assem.String(), unpacked.CharBank), nil
}

// EqualIgnoringLineSplits reports whether a and b differ at most in where
// their multi-line keep and delete runs are split, and in keeps without
// attributes at their end (etherpad-lite's equalIgnoringLineSplits): a
// retransmitted changeset is recognised against the line-split form the
// server stored.
func EqualIgnoringLineSplits(a, b string) bool {
	if a == b {
		return true
	}
	ua, errA := Unpack(a)
	ub, errB := Unpack(b)
	if errA != nil || errB != nil {
		return false
	}
	if ua.OldLen != ub.OldLen || ua.NewLen != ub.NewLen || ua.CharBank != ub.CharBank {
		return false
	}
	merged := func(s string) ([]Op, bool) {
		ops, err := DeserializeOps(s)
		if err != nil {
			return nil, false
		}
		out := make([]Op, 0, len(*ops))
		for _, op := range *ops {
			if n := len(out); n > 0 && op.OpCode != "+" && out[n-1].OpCode == op.OpCode && out[n-1].Attribs == op.Attribs {
				out[n-1].Chars += op.Chars
				out[n-1].Lines += op.Lines
				continue
			}
			out = append(out, op)
		}
		// Keeps without attributes at the end change nothing; the
		// assembler drops them, so the stored form may lack them.
		for len(out) > 0 && out[len(out)-1].OpCode == "=" && out[len(out)-1].Attribs == "" {
			out = out[:len(out)-1]
		}
		return out, true
	}
	ma, okA := merged(ua.Ops)
	mb, okB := merged(ub.Ops)
	if !okA || !okB || len(ma) != len(mb) {
		return false
	}
	for i := range ma {
		if ma[i] != mb[i] {
			return false
		}
	}
	return true
}
