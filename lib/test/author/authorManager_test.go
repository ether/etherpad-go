package author

import (
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

func TestAuthorManager(t *testing.T) {
	testDBHandler := testutils.NewTestDBHandler(t)
	defer testDBHandler.StartTestDBHandler()
	testDBHandler.AddTests(testutils.TestRunConfig{
		Name: "TestSetAuthorColor",
		Test: testSetAuthorColor,
	},
		testutils.TestRunConfig{
			Name: "TestCreateAuthor",
			Test: testCreateAuthor,
		},
		testutils.TestRunConfig{
			Name: "TestGetAuthorName",
			Test: testGetAuthorName,
		},
		testutils.TestRunConfig{
			Name: "TestGetAuthorName_NotFound",
			Test: testGetauthornameNotfound,
		},
		testutils.TestRunConfig{
			Name: "TestSetAuthorName",
			Test: testSetAuthorName,
		},
		testutils.TestRunConfig{
			Name: "TestGetAuthor4Token_NewToken",
			Test: testGetAuthor4Token_NewToken,
		},
		testutils.TestRunConfig{
			Name: "TestGetPadsOfAuthor",
			Test: testGetPadsOfAuthor,
		},
		testutils.TestRunConfig{
			Name: "TestAnonymizeAuthor",
			Test: testAnonymizeAuthor,
		},
		testutils.TestRunConfig{
			Name: "TestAnonymizeAuthor_UnknownAuthor",
			Test: testAnonymizeAuthorUnknown,
		},
		testutils.TestRunConfig{
			Name: "TestAnonymizeAuthor_Idempotent",
			Test: testAnonymizeAuthorIdempotent,
		},
	)
}

func testSetAuthorColor(t *testing.T, dbHandler testutils.TestDataStore) {
	randomAuthor := testutils.GenerateDBAuthor()
	err := dbHandler.DS.SaveAuthor(randomAuthor)
	if err != nil {
		t.Fatalf("Failed to save author: %v", err)
	}
	assert.NoError(t, dbHandler.AuthorManager.SetAuthorColor(randomAuthor.ID, "#123456"))
	savedAuthor, err := dbHandler.AuthorManager.GetAuthor(randomAuthor.ID)
	if err != nil {
		t.Fatalf("Failed to get author: %v", err)
	}
	assert.Equal(t, "#123456", savedAuthor.ColorId)
}

func testCreateAuthor(t *testing.T, dbHandler testutils.TestDataStore) {

	name := "alice"
	author, err := dbHandler.AuthorManager.CreateAuthor(&name)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if author.Id == "" {
		t.Fatalf("author id not set")
	}

	dbAuthor, err := dbHandler.AuthorManager.GetAuthor(author.Id)
	if err != nil {
		t.Fatalf("author not stored in db")
	}

	if dbAuthor.Name == nil || *dbAuthor.Name != name {
		t.Fatalf("author name not stored")
	}
}

func testGetAuthorName(t *testing.T, dbHandler testutils.TestDataStore) {

	name := "bob"
	author, _ := dbHandler.AuthorManager.CreateAuthor(&name)

	res, err := dbHandler.AuthorManager.GetAuthorName(author.Id)
	if err != nil {
		t.Fatalf("unexpected error")
	}

	if *res != name {
		t.Fatalf("expected %s, got %s", name, *res)
	}
}

func testGetauthornameNotfound(t *testing.T, dbHandler testutils.TestDataStore) {
	_, err := dbHandler.AuthorManager.GetAuthorName("does-not-exist")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func testSetAuthorName(t *testing.T, dbHandler testutils.TestDataStore) {

	author, err := dbHandler.AuthorManager.CreateAuthor(nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}

	err = dbHandler.AuthorManager.SetAuthorName(author.Id, "charlie")
	if err != nil {
		t.Fatalf("unexpected error")
	}

	dbAuthor, _ := dbHandler.AuthorManager.GetAuthor(author.Id)
	if dbAuthor.Name == nil || *dbAuthor.Name != "charlie" {
		t.Fatalf("name not updated")
	}
}

func testGetAuthor4Token_NewToken(t *testing.T, dbHandler testutils.TestDataStore) {

	author, err := dbHandler.AuthorManager.GetAuthor4Token("token-1")
	if err != nil {
		t.Fatalf("unexpected error")
	}

	if author.Id == "" {
		t.Fatalf("author id not set")
	}

	mapped, err := dbHandler.AuthorManager.GetAuthor4Token("token-1")
	if err != nil || mapped.Id != author.Id {
		t.Fatalf("token not mapped to author")
	}
}

func testGetPadsOfAuthor(t *testing.T, dbHandler testutils.TestDataStore) {

	author, _ := dbHandler.AuthorManager.CreateAuthor(nil)
	randomPad := db.CreateRandomPad()
	assert.NoError(t, dbHandler.DS.CreatePad(randomPad.ID, randomPad))
	if err := dbHandler.DS.SaveRevision(randomPad.ID, 0, "changeset0", db2.AText{
		Text:    randomPad.ATextText,
		Attribs: randomPad.ATextAttribs,
	}, db2.RevPool{
		NumToAttrib: randomPad.Pool.NumToAttrib,
		NextNum:     randomPad.Pool.NextNum,
	}, &author.Id, 12345); err != nil {
		t.Fatalf("SaveRevision failed: %v", err)
	}
	pads, err := dbHandler.AuthorManager.GetPadsOfAuthor(author.Id)
	if err != nil {
		t.Fatalf("unexpected error")
	}

	if len(*pads) != 1 || (*pads)[0] != randomPad.ID {
		t.Fatalf("unexpected pads")
	}
}

// testAnonymizeAuthor mirrors the original Etherpad's
// AuthorManager.anonymizeAuthor (GDPR Art. 17 erasure): the display identity
// (name, color) is zeroed, the token binding that links a person to the
// author id is severed, and authorship on chat messages is nulled while the
// message text itself is preserved. Pad content and revisions stay intact.
func testAnonymizeAuthor(t *testing.T, dbHandler testutils.TestDataStore) {
	name := "Alice GDPR"
	createdAuthor, err := dbHandler.AuthorManager.CreateAuthor(&name)
	if err != nil {
		t.Fatalf("failed to create author: %v", err)
	}
	assert.NoError(t, dbHandler.AuthorManager.SetAuthorColor(createdAuthor.Id, "#aabbcc"))

	token := "anonymize-token-1"
	assert.NoError(t, dbHandler.DS.SetAuthorByToken(token, createdAuthor.Id))

	// Author writes to a pad and posts a chat message.
	padText := "GDPR pad text\n"
	padId := "anonymizePad"
	_, err = dbHandler.PadManager.GetPad(padId, &padText, &createdAuthor.Id)
	if err != nil {
		t.Fatalf("failed to create pad: %v", err)
	}
	assert.NoError(t, dbHandler.DS.SaveChatMessage(padId, 0, &createdAuthor.Id, 12345, "my secret chat message"))

	assert.NoError(t, dbHandler.AuthorManager.AnonymizeAuthor(createdAuthor.Id))

	// Display identity is zeroed (name -> nil, colorId -> "0").
	scrubbed, err := dbHandler.AuthorManager.GetAuthor(createdAuthor.Id)
	if err != nil {
		t.Fatalf("anonymized author record must still exist: %v", err)
	}
	assert.Nil(t, scrubbed.Name, "name must be scrubbed")
	assert.Equal(t, "0", scrubbed.ColorId, "colorId must be zeroed")

	// Token binding is severed, the token can no longer resolve the author.
	resolved, err := dbHandler.DS.GetAuthorByToken(token)
	assert.Error(t, err, "token must no longer resolve to the author, got %v", resolved)

	// Chat message survives but its authorship is nulled.
	chats, err := dbHandler.DS.GetChatsOfPad(padId, 0, 0)
	if err != nil {
		t.Fatalf("failed to load chats: %v", err)
	}
	if len(*chats) != 1 {
		t.Fatalf("chat message must survive anonymization, got %d messages", len(*chats))
	}
	chat := (*chats)[0]
	assert.Nil(t, chat.AuthorId, "chat authorship must be nulled")
	assert.Nil(t, chat.DisplayName, "chat display name must be gone")
	assert.Equal(t, "my secret chat message", chat.Message, "chat text is preserved")
}

func testAnonymizeAuthorUnknown(t *testing.T, dbHandler testutils.TestDataStore) {
	err := dbHandler.AuthorManager.AnonymizeAuthor("a.doesNotExist123456")
	if err == nil {
		t.Fatalf("expected error for unknown author")
	}
	assert.Equal(t, db.AuthorNotFoundError, err.Error())
}

func testAnonymizeAuthorIdempotent(t *testing.T, dbHandler testutils.TestDataStore) {
	name := "Bob GDPR"
	createdAuthor, err := dbHandler.AuthorManager.CreateAuthor(&name)
	if err != nil {
		t.Fatalf("failed to create author: %v", err)
	}

	assert.NoError(t, dbHandler.AuthorManager.AnonymizeAuthor(createdAuthor.Id))
	// The original is idempotent: a second call succeeds and leaves the
	// record in the same erased state.
	assert.NoError(t, dbHandler.AuthorManager.AnonymizeAuthor(createdAuthor.Id))

	scrubbed, err := dbHandler.AuthorManager.GetAuthor(createdAuthor.Id)
	if err != nil {
		t.Fatalf("anonymized author record must still exist: %v", err)
	}
	assert.Nil(t, scrubbed.Name)
	assert.Equal(t, "0", scrubbed.ColorId)
}
