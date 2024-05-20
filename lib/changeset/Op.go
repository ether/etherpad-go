package changeset

import (
	"github.com/ether/etherpad-go/lib/utils"
)

type Op struct {
	OpCode  string
	Chars   int
	Lines   int
	Attribs string
}

func NewOp(opCode *string) Op {
	var newOpCode string
	if opCode == nil {
		newOpCode = ""
	} else {
		newOpCode = *opCode
	}
	var chars = 0
	var lines = 0
	var attribs = ""
	return Op{
		OpCode:  newOpCode,
		Chars:   chars,
		Lines:   lines,
		Attribs: attribs,
	}
}

func (op *Op) String() string {
	var l string
	if op.Lines == 0 {
		l = ""
	} else {
		l = utils.NumToString(op.Lines)
	}

	return op.Attribs + l + op.OpCode + utils.NumToString(op.Chars)
}

func copyOp(op1 Op, op2 *Op) *Op {
	if op2 != nil {
		op2.OpCode = op1.OpCode
		op2.Chars = op1.Chars
		op2.Lines = op1.Lines
		op2.Attribs = op1.Attribs
	} else {
		op2 = &Op{
			OpCode:  op1.OpCode,
			Chars:   op1.Chars,
			Lines:   op1.Lines,
			Attribs: op1.Attribs,
		}
		return op2
	}
	return nil
}

func (op *Op) clearOp() {
	op.OpCode = ""
	op.Chars = 0
	op.Lines = 0
	op.Attribs = ""
}
