package changeset

type SmartOpAssembler struct {
	minusAssem   MergingOpAssembler
	plusAssem    MergingOpAssembler
	keepAssem    MergingOpAssembler
	assem        StringAssembler
	lastOpcode   string
	lengthChange int
}

func NewSmartOpAssembler() *SmartOpAssembler {
	return &SmartOpAssembler{
		minusAssem:   NewMergingOpAssembler(),
		plusAssem:    NewMergingOpAssembler(),
		keepAssem:    NewMergingOpAssembler(),
		assem:        NewStringAssembler(),
		lastOpcode:   "",
		lengthChange: 0,
	}
}

func (sm *SmartOpAssembler) FlushKeeps() {
	sm.assem.Append(sm.keepAssem.String())
	sm.keepAssem.Clear()
}

func (sm *SmartOpAssembler) flushPlusMinus() {
	sm.assem.Append(sm.minusAssem.String())
	sm.minusAssem.Clear()
	sm.assem.Append(sm.plusAssem.String())
	sm.plusAssem.Clear()
}

func (sm *SmartOpAssembler) Append(op Op) {
	if op.OpCode == "" {
		return
	}

	if op.Chars == 0 {
		return
	}

	if op.OpCode == "-" {
		if sm.lastOpcode == "=" {
			sm.FlushKeeps()
		}
		sm.minusAssem.Append(op)
		sm.lengthChange -= op.Chars
	} else if op.OpCode == "+" {
		if sm.lastOpcode == "=" {
			sm.FlushKeeps()
		}
		sm.plusAssem.Append(op)
		sm.lengthChange += op.Chars
	} else if op.OpCode == "" {
		if sm.lastOpcode != "=" {
			sm.flushPlusMinus()
		}
		sm.keepAssem.Append(op)
	}

	sm.lastOpcode = op.OpCode
}

func (sm *SmartOpAssembler) String() string {
	sm.flushPlusMinus()
	sm.FlushKeeps()
	return sm.assem.String()
}

func (sm *SmartOpAssembler) Clear() {
	sm.minusAssem.Clear()
	sm.plusAssem.Clear()
	sm.keepAssem.Clear()
	sm.assem.Clear()
	sm.lengthChange = 0
}

func (sm *SmartOpAssembler) EndDocument() {
	sm.keepAssem.EndDocument()
}

func (sm *SmartOpAssembler) LengthChange() int {
	return sm.lengthChange
}
