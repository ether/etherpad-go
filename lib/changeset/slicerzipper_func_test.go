package changeset

import (
	"github.com/ether/etherpad-go/lib/apool"
	"testing"
)

func TestWithEmptyAttOpEmpty(t *testing.T) {
	var opCode = ""
	var attrOp Op = NewOp(&opCode)
	var opCodeCS = "+"
	var csOp = NewOp(&opCodeCS)
	var pool = apool.NewAPool()
	var op, err = SlicerZipperFunc(&attrOp, &csOp, *pool)

	if op.OpCode != "+" || csOp.OpCode != "" || err != nil {
		t.Error("Both should be empty")
	}
}

func TestWithEmptyCsOp(t *testing.T) {
	var attOpCode = "+"
	var attrOp Op = NewOp(&attOpCode)
	var opCodeCS = ""
	var csOp = NewOp(&opCodeCS)
	var pool = apool.NewAPool()
	var op, err = SlicerZipperFunc(&attrOp, &csOp, *pool)

	if attrOp.OpCode != "" || err != nil || op.OpCode != "+" {
		t.Error("Opcode should be empty")
	}
}

func TestWithMinusAttOpCsOp(t *testing.T) {
	var attOpCode = "-"
	var attrOp Op = NewOp(&attOpCode)
	var opCodeCS = ""
	var csOp = NewOp(&opCodeCS)
	var pool = apool.NewAPool()
	var _, _ = SlicerZipperFunc(&attrOp, &csOp, *pool)

	if attrOp.OpCode != "" {
		t.Error("Opcode should be empty")
	}
}

func TestWithPlusCSOp(t *testing.T) {
	var attOpCode = ""
	var attrOp Op = NewOp(&attOpCode)
	var opCodeCS = "+"
	var csOp = NewOp(&opCodeCS)
	var pool = apool.NewAPool()
	var opout, _ = SlicerZipperFunc(&attrOp, &csOp, *pool)

	if csOp.OpCode != "" || opout.OpCode != "+" {
		t.Error("Opcode should be empty")
	}
}
