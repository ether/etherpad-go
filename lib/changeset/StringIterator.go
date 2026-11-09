package changeset

import (
	"errors"
	"strings"

	"github.com/ether/etherpad-go/lib/utils"
)

// StringIterator walks a string rune by rune. It keeps byte and rune
// positions, so Take and Peek return substrings of the original string
// without converting it to []rune.
type StringIterator struct {
	curIndex int // rune position
	bytePos  int
	runeLen  int
	newLines int
	str      string
}

func NewStringIterator(str string) StringIterator {
	return StringIterator{
		runeLen:  utils.RuneCount(str),
		newLines: strings.Count(str, "\n"),
		str:      str,
	}
}

func (si *StringIterator) Remaining() int {
	return si.runeLen - si.curIndex
}

func (si *StringIterator) AssertRemaining(n int) error {
	if n > si.Remaining() {
		return errors.New("not enough characters remaining")
	}
	return nil
}

// end returns the byte offset n runes after the current position.
func (si *StringIterator) end(n int) int {
	if si.runeLen == len(si.str) { // ASCII only
		return si.bytePos + n
	}
	return si.bytePos + utils.RuneByteOffset(si.str[si.bytePos:], n)
}

func (si *StringIterator) Take(n int) (*string, error) {
	if err := si.AssertRemaining(n); err != nil {
		return nil, err
	}
	e := si.end(n)
	s := si.str[si.bytePos:e]
	si.newLines -= strings.Count(s, "\n")
	si.bytePos = e
	si.curIndex += n
	return &s, nil
}

func (si *StringIterator) Peek(n int) (*string, error) {
	if err := si.AssertRemaining(n); err != nil {
		return nil, err
	}
	s := si.str[si.bytePos:si.end(n)]
	return &s, nil
}

func (si *StringIterator) Skip(n int) error {
	if err := si.AssertRemaining(n); err != nil {
		return err
	}
	si.bytePos = si.end(n)
	si.curIndex += n
	return nil
}
