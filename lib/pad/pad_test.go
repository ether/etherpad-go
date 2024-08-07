package pad

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestCleanText(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"\n", "\n"},
		{"x", "x"},
		{"x\n", "x\n"},
		{"x\ny\n", "x\ny\n"},
		{"x\ry\n", "x\ny\n"},
		{"x\r\ny\n", "x\ny\n"},
		{"x\r\r\ny\n", "x\n\ny\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := pad.CleanText(tc.input)
			if *got != tc.want {
				t.Errorf("CleanText(%q) = %q; want %q", tc.input, *got, tc.want)
			}
		})
	}
}

func TestPadDefaultingToSettingsText(t *testing.T) {
	var padAuthor = author.Author{
		"123",
		nil,
		"1",
		make(map[string]struct{}),
		123,
	}
	manager := NewManager()
	var pad, _ = manager.GetPad("test", nil, &padAuthor)
	var padText = settings.SettingsDisplayed.DefaultPadText

	if pad.AText.Text != padText {
		t.Error("Error setting pad text to default pad text")
	}
}

func TestApplyToAText(t *testing.T) {
	var pool = apool.NewAPool()
	var newText = changeset.ApplyToAText("Z:1>j+j$Welcome to Etherpad", apool.AText{
		Text:    "\n",
		Attribs: "|1+1",
	}, *pool)
	if newText.Text != "Welcome to Etherpad\n" || newText.Attribs != "|1+k" {
		t.Error("Error ", newText.Attribs)
	}
}

func TestUnpack(t *testing.T) {
	var pool = apool.NewAPool()
	var unpacked, err = changeset.Unpack("Z:1>j+j$Welcome to Etherpad")
	if err != nil {
		t.Error("Error unpacking changeset")
	}
	if unpacked.OldLen != 1 || unpacked.NewLen != 20 || unpacked.Ops != "+j" || unpacked.CharBank != "Welcome to Etherpad" {
		t.Error("Error unpacking")
	}
	var counter = 0

	var firstOps = []changeset.Op{
		{
			OpCode:  "+",
			Chars:   1,
			Lines:   1,
			Attribs: "",
		},
		{
			OpCode:  "+",
			Chars:   19,
			Lines:   0,
			Attribs: "",
		},
	}

	var secondOps = []changeset.Op{
		{
			OpCode:  "+",
			Chars:   1,
			Lines:   1,
			Attribs: "",
		},
		{
			OpCode:  "",
			Chars:   0,
			Lines:   0,
			Attribs: "",
		},
	}

	var changes = [][]changeset.Op{
		firstOps,
		secondOps,
	}

	var slicerResults = []changeset.Op{
		{
			OpCode:  "+",
			Chars:   19,
			Lines:   0,
			Attribs: "",
		},
		{
			OpCode:  "+",
			Chars:   1,
			Lines:   1,
			Attribs: "",
		},
	}

	changeset.ApplyZip("|1+1", unpacked.Ops, func(op *changeset.Op, op2 *changeset.Op) changeset.Op {
		if counter == 2 {
			t.Error("Should only iterate twice")
			panic("Error syncing")
		}

		if !cmp.Equal(changes[counter][0], *op) || !cmp.Equal(changes[counter][1], *op2) {
			t.Error("Error comparing applyzip")
		}

		var slicer, _ = changeset.SlicerZipperFunc(op, op2, *pool)
		var slicerREsult = slicerResults[counter]
		if !cmp.Equal(slicerREsult, *slicer) {
			t.Error("Error comparing slicer")
		}
		counter += 1

		return *slicer
	})

}
