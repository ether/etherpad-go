package changeset

import (
	"errors"

	"github.com/ether/etherpad-go/lib/utils"
)

type StringIterator struct {
	curIndex int
	newLines int
	str      []rune
}

func NewStringIterator(str string) StringIterator {
	rs := []rune(str)
	return StringIterator{curIndex: 0, newLines: utils.CountLines(str, '\n'), str: rs}
}

func (si *StringIterator) Remaining() int {
	return len(si.str) - si.curIndex
}

func (si *StringIterator) AssertRemaining(n int) error {
	if n > si.Remaining() {
		return errors.New("not enough characters remaining")
	}
	return nil
}

func (si *StringIterator) Take(n int) string {
	if err := si.AssertRemaining(n); err != nil {
		panic(err)
	}
	segment := si.str[si.curIndex : si.curIndex+n]
	s := string(segment)
	si.newLines -= countNewlines(segment)
	si.curIndex += n
	return s
}

func (si *StringIterator) Peek(n int) string {
	if err := si.AssertRemaining(n); err != nil {
		panic(err)
	}
	return string(si.str[si.curIndex : si.curIndex+n])
}

func (si *StringIterator) Skip(n int) error {
	if err := si.AssertRemaining(n); err != nil {
		return err
	}
	si.curIndex += n
	return nil
}

func countNewlines(rs []rune) int {
	c := 0
	for _, r := range rs {
		if r == '\n' {
			c++
		}
	}
	return c
}
