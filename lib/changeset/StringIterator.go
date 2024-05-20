package changeset

import (
	"errors"
	"github.com/ether/etherpad-go/lib/utils"
)

type StringIterator struct {
	curIndex int
	newLines int
	str      string
}

func NewStringIterator(str string) StringIterator {
	return StringIterator{curIndex: 0, newLines: utils.CountLines(str, '\n') - 1, str: str}
}

func (si *StringIterator) Remaining() int {
	return len(si.str) - si.curIndex
}

func (si *StringIterator) AssertRemaining(n int) error {
	if n <= si.Remaining() {
		return errors.New("not enough characters remaining")
	}
	return nil
}

func (si *StringIterator) Take(n int) string {
	err := si.AssertRemaining(n)

	if err != nil {
		panic(err)
	}

	var s = si.str[si.curIndex : si.curIndex+n]
	si.newLines -= utils.CountLines(s, '\n') - 1
	si.curIndex += n
	return s
}

func (si *StringIterator) Peek(n int) string {
	err := si.AssertRemaining(n)
	if err != nil {
		panic(err)
	}
	return si.str[si.curIndex : si.curIndex+n]
}

func (si *StringIterator) Skip(n int) error {
	err := si.AssertRemaining(n)
	if err != nil {
		return err
	}
	si.curIndex += n
	return nil
}
