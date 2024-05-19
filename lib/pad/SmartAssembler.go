package pad

type StringAssembler struct {
	str string
}

func NewStringAssembler() StringAssembler {
	return StringAssembler{
		str: "",
	}
}

func (sa *StringAssembler) Append(s string) {
	sa.str += s
}

func (sa *StringAssembler) String() string {
	return sa.str
}

func (sa *StringAssembler) Clear() {
	sa.str = ""
}
