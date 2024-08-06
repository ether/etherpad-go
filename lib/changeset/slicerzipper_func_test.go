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

func TestWithValues(t *testing.T) {
	var ops = make([][]Op, 0)
	var firstInsert = make([]Op, 0)
	var secondInsert = make([]Op, 0)
	var thirdInsert = make([]Op, 0)
	var fourthInsert = make([]Op, 0)
	var fifthInsert = make([]Op, 0)
	var sixthInsert = make([]Op, 0)

	var results = make([]Op, 0)

	firstInsert = append(firstInsert, Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   0,
		Attribs: "",
	})
	firstInsert = append(firstInsert, Op{
		OpCode:  "-",
		Chars:   1,
		Lines:   0,
		Attribs: "",
	})

	secondInsert = append(secondInsert, Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   0,
		Attribs: "*1",
	})

	secondInsert = append(secondInsert, Op{
		OpCode:  "=",
		Chars:   1,
		Lines:   0,
		Attribs: "*0",
	})

	thirdInsert = append(thirdInsert, Op{
		OpCode:  "+",
		Chars:   5,
		Lines:   1,
		Attribs: "",
	})

	thirdInsert = append(thirdInsert, Op{
		OpCode:  "=",
		Chars:   1,
		Lines:   0,
		Attribs: "*1",
	})

	fourthInsert = append(fourthInsert, Op{
		OpCode:  "+",
		Chars:   4,
		Lines:   1,
		Attribs: "",
	})

	fourthInsert = append(fourthInsert, Op{
		OpCode:  "=",
		Chars:   3,
		Lines:   0,
		Attribs: "",
	})

	fifthInsert = append(fifthInsert, Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   1,
		Attribs: "",
	})

	fifthInsert = append(fifthInsert, Op{
		OpCode:  "+",
		Chars:   4,
		Lines:   0,
		Attribs: "",
	})

	sixthInsert = append(sixthInsert, Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   1,
		Attribs: "",
	})

	sixthInsert = append(sixthInsert, Op{
		OpCode:  "",
		Chars:   0,
		Lines:   0,
		Attribs: "",
	})

	results = append(results, Op{
		OpCode:  "",
		Chars:   1,
		Lines:   0,
		Attribs: "",
	})

	results = append(results, Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   0,
		Attribs: "",
	})

	results = append(results, Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   0,
		Attribs: "*1",
	})

	results = append(results, Op{
		OpCode:  "+",
		Chars:   3,
		Lines:   0,
		Attribs: "",
	})

	results = append(results, Op{
		OpCode:  "+",
		Chars:   4,
		Lines:   0,
		Attribs: "",
	})

	results = append(results, Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   1,
		Attribs: "",
	})

	ops = append(ops, firstInsert)
	ops = append(ops, secondInsert)
	ops = append(ops, thirdInsert)
	ops = append(ops, fourthInsert)
	ops = append(ops, fifthInsert)
	ops = append(ops, sixthInsert)

	var mapAttrib = make(map[int]apool.Attribute)
	var attribToNum = make(map[apool.Attribute]int)

	mapAttrib[0] = apool.Attribute{
		Key:   "bold",
		Value: "",
	}

	mapAttrib[1] = apool.Attribute{
		Key:   "bold",
		Value: "true",
	}

	attribToNum[apool.Attribute{
		Key:   "bold",
		Value: "0",
	}] = 0

	attribToNum[apool.Attribute{
		Key:   "bold",
		Value: "true",
	}] = 1

	var pool = apool.APool{
		NumToAttrib: mapAttrib,
		AttribToNum: attribToNum,
	}
	for i, opsRetrieved := range ops {
		zipperFunc, err := SlicerZipperFunc(&opsRetrieved[0], &opsRetrieved[1], pool)
		if err != nil {
			t.Error("Error creating zip")
		}

		if zipperFunc.Chars != results[i].Chars ||
			zipperFunc.Lines != results[i].Lines ||
			zipperFunc.OpCode != results[i].OpCode {
			t.Error("error syncing zip and result", zipperFunc, results[i])
		}

	}
}
