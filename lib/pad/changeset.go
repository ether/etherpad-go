package pad

import (
	"errors"
	"github.com/ether/etherpad-go/lib/models/pad"
	"strings"
)

func opsFromText(opcode string, text string, attribs string, pool *pad.APool) <-chan Op {
	ch := make(chan Op)

	go func() {
		defer close(ch)

		op := NewOp(&opcode)
		if attribs != "" {
			op.Attribs = attribs
		} else if pool != nil {

			op.Attribs = pool.Update([]string{}, opcode == "+").ToString()
		}

		lastNewlinePos := strings.LastIndex(text, "\n")
		if lastNewlinePos < 0 {
			op.Chars = len(text)
			op.Lines = 0
			ch <- op
		} else {
			op.Chars = lastNewlinePos + 1
			op.Lines = strings.Count(text, "\n")
			ch <- op

			op2 := copyOp(op)
			op2.Chars = len(text) - (lastNewlinePos + 1)
			op2.Lines = 0
			ch <- op2
		}
	}()

	return ch
}

func ops(orig string, start int, deleted string, ins string, attribs string, pool pad.APool) <-chan Op {
	ch := make(chan Op)

	go func() {
		defer close(ch)

		// opsFromText('=', orig.substring(0, start));
		for _, op := range opsFromText("=", orig[:start]) {
			ch <- op
		}

		// opsFromText('-', deleted);
		for _, op := range opsFromText("-", deleted) {
			ch <- op
		}

		// opsFromText('+', ins, attribs, pool);
		for _, op := range opsFromText("+", ins, attribs, pool) {
			ch <- op
		}
	}()

	return ch
}

func MakeSplice(orig string, start int, ndel int, ins string, attribs string, pool pad.APool) (string, error) {
	if start < 0 {
		return "", errors.New("start is negative")
	}

	if ndel < 0 {
		return "", errors.New("ndel is negative")
	}

	if start > len(orig) {
		start = len(orig)
	}

	if ndel > len(orig)-start {
		ndel = len(orig) - start
	}

	var deleted = orig[start : start+ndel]
	var assem = NewSmartOpAssembler()

}
