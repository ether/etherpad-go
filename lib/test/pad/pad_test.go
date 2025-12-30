package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/models/revision"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
)

func TestPad(t *testing.T) {
	testHandler := testutils.NewTestDBHandler(t)
	testHandler.AddTests(testutils.TestRunConfig{
		Name: "Create Pad with White Space in Pad ID",
		Test: testCheckWithWhiteSpaceInPadID,
	},
		testutils.TestRunConfig{
			Name: "Create Pad with Negative Head",
			Test: testCheckWithNegativeHead,
		},
		testutils.TestRunConfig{
			Name: "Create Pad with Different Saved Revision Numbers",
			Test: testDifferentSavedRevisionNumbers,
		},
		testutils.TestRunConfig{
			Name: "Append Chat Message to Pad",
			Test: testAppendChatMessage,
		},
		testutils.TestRunConfig{
			Name: "Append Chat Message to Non-Existing Pad",
			Test: testAppendChatMessageOnNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "Get Revision Changesets on Non-Existing Pad",
			Test: testGetRevisionChangesetsOnNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "Get Internal Revision AText",
			Test: testGetInternalRevisionAText,
		},
		testutils.TestRunConfig{
			Name: "Get Revision Changesets on Existing Pad non-Existing Revision",
			Test: testGetRevisionChangesetsOnExistingPadNonExistingRevision,
		},
	)

	defer testHandler.StartTestDBHandler()
}

func testCheckWithWhiteSpaceInPadID(t *testing.T, ts testutils.TestDataStore) {
	padToTest := pad.CreateNewPad(ts.DS)
	padToTest.Id = "pad with spaces"
	if err := padToTest.Check(); err == nil || err.Error() != "pad id contains leading or trailing whitespace" {
		t.Fatal("should fail with whitespaces" + err.Error())
	}
}

func testCheckWithNegativeHead(t *testing.T, ts testutils.TestDataStore) {
	padToTest := pad.CreateNewPad(ts.DS)
	padToTest.Head = -1
	if err := padToTest.Check(); err == nil || err.Error() != "pad head revision is negative" {
		t.Fatal("should fail with negative head", err)
	}
}

func testDifferentSavedRevisionNumbers(t *testing.T, ts testutils.TestDataStore) {
	padToTest := pad.CreateNewPad(ts.DS)
	padToTest.Head = 1
	padToTest.SavedRevisions = append(padToTest.SavedRevisions, revision.SavedRevision{
		RevNum: 25,
	})
	padToTest.SavedRevisions = append(padToTest.SavedRevisions, revision.SavedRevision{
		RevNum: 23,
	})
	if err := padToTest.Check(); err == nil || err.Error() != "pad saved revisions are not in ascending order" {
		t.Fatal("should fail with different saved revision numbers", err)
	}
}

func testAppendChatMessage(t *testing.T, ts testutils.TestDataStore) {
	padToTest := pad.CreateNewPad(ts.DS)
	padToTest.Head = 1
	if err := padToTest.Save(); err != nil {
		t.Fatal("failed to save pad:", err)
	}
	randomAuthor := author.NewRandomAuthor()
	createdAuthor, err := ts.AuthorManager.CreateAuthor(randomAuthor.Name)
	if err != nil {
		t.Fatal("failed to create author:", err)
	}

	padHead, err := padToTest.AppendChatMessage(&createdAuthor.Id, 1234567890, "Hello, world!")
	if err != nil {
		t.Fatal("failed to append chat message:", err)
	}

	if *padHead != 0 {
		t.Fatal("expected pad head to be 1 after appending chat message, got:", *padHead)
	}
}

func testAppendChatMessageOnNonExistingPad(t *testing.T, ts testutils.TestDataStore) {
	padToTest := pad.CreateNewPad(ts.DS)
	randomAuthor := author.NewRandomAuthor()
	createdAuthor, err := ts.AuthorManager.CreateAuthor(randomAuthor.Name)
	if err != nil {
		t.Fatal("failed to create author:", err)
	}

	_, err = padToTest.AppendChatMessage(&createdAuthor.Id, 1234567890, "Hello, world!")
	if err == nil {
		t.Fatal("should error with non existing pad:", err)
	}
}

func testGetRevisionChangesetsOnNonExistingPad(t *testing.T, ts testutils.TestDataStore) {
	padToTest := pad.CreateNewPad(ts.DS)

	_, err := padToTest.GetRevisionChangeset(100)
	if err == nil {
		t.Fatal("should error with non existing pad:", err)
	}
}

func testGetInternalRevisionAText(t *testing.T, ts testutils.TestDataStore) {
	padToTest := pad.CreateNewPad(ts.DS)
	gotPad, err := ts.PadManager.GetPad(padToTest.Id, nil, nil)
	if err != nil {
		t.Fatal("failed to get pad:", err)
	}

	apool := gotPad.GetInternalRevisionAText(0)
	if apool == nil {
		t.Fatal("expected a valid AText, got nil")
	}
}

func testGetRevisionChangesetsOnExistingPadNonExistingRevision(t *testing.T, ts testutils.TestDataStore) {
	padToTest := pad.CreateNewPad(ts.DS)
	gotPad, err := ts.PadManager.GetPad(padToTest.Id, nil, nil)
	if err != nil {
		t.Fatal("failed to get pad:", err)
	}

	_, err = gotPad.GetRevisionChangeset(100)
	if err == nil || err.Error() != "pad revision not found" {
		t.Fatal("should work with existing pad:", err)
	}
}
