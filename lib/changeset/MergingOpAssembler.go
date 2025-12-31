package changeset

type MergingOpAssembler struct {
	assem                            OpAssembler
	bufOp                            Op
	bufOpAdditionalCharsAfterNewline int
}

func NewMergingOpAssembler() *MergingOpAssembler {
	return &MergingOpAssembler{
		assem: NewOpAssembler(),
		bufOp: NewOp(nil),
	}
}

func (m *MergingOpAssembler) flush(isEndDocument bool) {
	if m.bufOp.OpCode == "" {
		return
	}
	if isEndDocument && m.bufOp.OpCode == "=" && m.bufOp.Attribs == "" {
		// final merged keep, leave it implicit
	} else {
		m.assem.Append(m.bufOp)
		if m.bufOpAdditionalCharsAfterNewline != 0 {
			m.bufOp.Chars = m.bufOpAdditionalCharsAfterNewline
			m.bufOp.Lines = 0
			m.assem.Append(m.bufOp)
			m.bufOpAdditionalCharsAfterNewline = 0
		}
	}
	m.bufOp.OpCode = ""
}

func (m *MergingOpAssembler) Append(op Op) {
	if op.Chars <= 0 {
		return
	}
	if m.bufOp.OpCode == op.OpCode && m.bufOp.Attribs == op.Attribs {
		if op.Lines > 0 {
			m.bufOp.Chars += m.bufOpAdditionalCharsAfterNewline + op.Chars
			m.bufOp.Lines += op.Lines
			m.bufOpAdditionalCharsAfterNewline = 0
		} else if m.bufOp.Lines == 0 {
			m.bufOp.Chars += op.Chars
		} else {
			m.bufOpAdditionalCharsAfterNewline += op.Chars
		}
	} else {
		m.flush(false)
		copyOp(op, &m.bufOp)
	}
}

func (m *MergingOpAssembler) EndDocument() {
	m.flush(true)
}

func (m *MergingOpAssembler) String() string {
	m.flush(false)
	return m.assem.String()
}

func (m *MergingOpAssembler) Clear() {
	m.assem.Clear()
	m.bufOp.clearOp()
	m.bufOpAdditionalCharsAfterNewline = 0
}
