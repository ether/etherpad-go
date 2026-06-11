package paddiff

import (
	"github.com/ether/etherpad-go/lib/changeset"
)

// opBuilder is a tiny local replacement for changeset.Builder. It is needed
// because changeset.Builder only accepts attributes through the unexported
// fields of changeset.KeepArgs, while the padDiff port has to pass already
// encoded attribute strings (e.g. "*3*4") to keep/insert operations. It uses
// the same SmartOpAssembler/Pack primitives as changeset.Builder, so the
// produced changesets are identical.
type opBuilder struct {
	oldLen   int
	assem    *changeset.SmartOpAssembler
	charBank changeset.StringAssembler
}

func newOpBuilder(oldLen int) *opBuilder {
	return &opBuilder{
		oldLen:   oldLen,
		assem:    changeset.NewSmartOpAssembler(),
		charBank: changeset.NewStringAssembler(),
	}
}

// keep appends a '=' op over n chars (l of them newlines) carrying the given
// already-encoded attribute string.
func (b *opBuilder) keep(n int, l int, attribs string) {
	opCode := "="
	op := changeset.NewOp(&opCode)
	op.Chars = n
	if l > 0 {
		op.Lines = l
	}
	op.Attribs = attribs
	b.assem.Append(op)
}

// keepText appends '=' ops covering the given text (which may contain
// newlines) carrying the given already-encoded attribute string.
func (b *opBuilder) keepText(text string, attribs string) {
	for _, op := range changeset.OpsFromText("=", text, nil, nil) {
		op.Attribs = attribs
		b.assem.Append(op)
	}
}

// insert appends '+' ops for the given text carrying the given
// already-encoded attribute string.
func (b *opBuilder) insert(text string, attribs string) {
	for _, op := range changeset.OpsFromText("+", text, nil, nil) {
		op.Attribs = attribs
		b.assem.Append(op)
	}
	b.charBank.Append(text)
}

// toString finalizes the assembler and packs the changeset.
func (b *opBuilder) toString() string {
	b.assem.EndDocument()
	newLen := b.oldLen + b.assem.LengthChange()
	return changeset.Pack(b.oldLen, newLen, b.assem.String(), b.charBank.String())
}
