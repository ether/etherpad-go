package pad

import (
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/google/go-cmp/cmp"
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
	var createdHooks = hooks.NewHook()
	memoryStore := db.NewMemoryDataStore()
	var padAuthor = author.Author{
		Id:        "123",
		ColorId:   "#FFFFFF",
		PadIDs:    make(map[string]struct{}),
		Timestamp: 123,
	}
	manager := NewManager(memoryStore, &createdHooks)
	var retrievedPad, _ = manager.GetPad("test", nil, &padAuthor.Id)
	var padText = settings.Displayed.DefaultPadText

	if retrievedPad.AText.Text != padText+"\n" {
		t.Error("Error setting pad text to default pad text")
	}
}

func TestUseProvidedContent(t *testing.T) {
	var createdHooks = hooks.NewHook()
	var want = "hello world"
	if want == settings.Displayed.DefaultPadText {
		return
	}
	createdHooks.EnqueueHook(hooks.PadDefaultContentString, func(ctx any) {
		var content = ctx.(*pad.DefaultContent)

		var emptyString = ""
		content.Content = &emptyString
		content.Content = &want
	})

	var padManager = NewManager(db.NewMemoryDataStore(), &createdHooks)
	var createdPad, _ = padManager.GetPad("test", nil, nil)
	var createdText = createdPad.Text()
	if strings.TrimSpace(createdText) != want {
		t.Error("Error modifying text " + createdText)
	}
}

func TestApplyToAText(t *testing.T) {
	var pool = apool.NewAPool()
	var newText, err = changeset.ApplyToAText("Z:1>j+j$Welcome to Etherpad", apool.AText{
		Text:    "\n",
		Attribs: "|1+1",
	}, pool)
	if err != nil {
		t.Error("Error applying to atext ", err)
		return
	}
	if newText.Text != "Welcome to Etherpad\n" || newText.Attribs != "|1+k" {
		t.Error("Error ", newText.Attribs)
	}
}

func TestRunWhenAPadIsCreated(t *testing.T) {
	var called = false
	var hook = hooks.NewHook()
	hook.EnqueueHook(hooks.PadDefaultContentString, func(ctx any) {
		called = true
	})
	var padManager = NewManager(db.NewMemoryDataStore(), &hook)
	var _, _ = padManager.GetPad("test", nil, nil)
	if !called {
		t.Error("Default pad content string hook should be called")
	}
}

func TestNotCalledWithSpecificText(t *testing.T) {
	var called = false
	var hook = hooks.NewHook()
	hook.EnqueueHook(hooks.PadDefaultContentString, func(ctx any) {
		called = true
	})
	var padManager = NewManager(db.NewMemoryDataStore(), &hook)
	var padText = "test"
	var _, _ = padManager.GetPad("test", &padText, nil)
	if called {
		t.Error("Default pad content string hook should be called")
	}
}

func TestDefaultsToSettingsPadText(t *testing.T) {
	var hook = hooks.NewHook()
	var padManager = NewManager(db.NewMemoryDataStore(), &hook)
	hook.EnqueueHook(hooks.PadDefaultContentString, func(ctx any) {
		if *ctx.(*pad.DefaultContent).Type != "text" {
			t.Error("wrong type")
		}

		if *ctx.(*pad.DefaultContent).Content != settings.Displayed.DefaultPadText {
			t.Error("Default pad text should be settings pad text")
		}
	})

	padManager.GetPad("test", nil, nil)
}

func TestPassesEmptyAuthorIdIfNotProvided(t *testing.T) {
	var authorId *string
	var hook = hooks.NewHook()
	hook.EnqueueHook(hooks.PadDefaultContentString, func(ctx any) {
		authorId = ctx.(*pad.DefaultContent).AuthorId
	})
	padManager := NewManager(db.NewMemoryDataStore(), &hook)

	padManager.GetPad("test", nil, nil)
	if authorId != nil {
		t.Error("Author id should be empty")
	}
}

func TestPassesAuthorIdIfProvided(t *testing.T) {
	var hook = hooks.NewHook()
	var authorId string
	hook.EnqueueHook(hooks.PadDefaultContentString, func(ctx any) {
		authorId = *ctx.(*pad.DefaultContent).AuthorId
	})
	padManager := NewManager(db.NewMemoryDataStore(), &hook)

	var authorIdProvided = "123"
	padManager.GetPad("test", nil, &authorIdProvided)
	if authorId != "123" {
		t.Error("Author id should be 123")
	}
}

func TestUnpack(t *testing.T) {
	var pool = apool.NewAPool()
	var unpacked, err = changeset.Unpack("Z:1>j+j$Welcome to Etherpad")
	if err != nil {
		t.Error("Error unpacking changeset")
		return
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

	resString, err := changeset.ApplyZip("|1+1", unpacked.Ops, func(op *changeset.Op, op2 *changeset.Op) (*changeset.Op, error) {
		if counter == 2 {
			t.Error("Should only iterate twice")
			panic("Error syncing")
		}

		if !cmp.Equal(changes[counter][0], *op) || !cmp.Equal(changes[counter][1], *op2) {
			t.Error("Error comparing applyzip")
		}

		var slicer, _ = changeset.SlicerZipperFunc(op, op2, &pool)
		var slicerREsult = slicerResults[counter]
		if !cmp.Equal(slicerREsult, *slicer) {
			t.Error("Error comparing slicer")
		}
		counter += 1

		return slicer, nil
	})
	if err != nil {
		t.Error("Error applying zip", err)
		return
	}
	if *resString != "|1+k" {
		t.Error("Error applying zip result string")
		return
	}
}
