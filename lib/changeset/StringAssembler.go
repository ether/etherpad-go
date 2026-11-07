package changeset

import "strings"

type StringAssembler struct {
	b strings.Builder
}

func NewStringAssembler() StringAssembler {
	return StringAssembler{}
}

func (sa *StringAssembler) Append(s string) {
	sa.b.WriteString(s)
}

func (sa *StringAssembler) String() string {
	return sa.b.String()
}

func (sa *StringAssembler) Clear() {
	sa.b.Reset()
}
