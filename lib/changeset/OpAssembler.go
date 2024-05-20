package changeset

type OpAssembler struct {
	serialized string
}

func NewOpAssembler() OpAssembler {
	return OpAssembler{
		serialized: "",
	}
}

func (oa *OpAssembler) Append(op Op) {
	oa.serialized += op.String()
}

func (oa *OpAssembler) String() string {
	return oa.serialized
}

func (oa *OpAssembler) Clear() {
	oa.serialized = ""
}
