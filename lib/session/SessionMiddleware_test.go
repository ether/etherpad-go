package session

import (
	"github.com/ether/etherpad-go/lib/models/session"
	"github.com/ether/etherpad-go/lib/utils"
	"testing"
	"time"
)

var sessionId string

func prepareTest() (*MemoryStore, string) {
	sessionId = utils.RandomString(10)
	sessionStore := NewMemoryStore(nil)
	return sessionStore, sessionId
}

func TestMemoryStore_SetOfNonExpiringSession(t *testing.T) {
	var memoryStore, sid = prepareTest()
	var sessionRetrieved = session.Session{
		Expires:        "2024-12-12T12:12:12.123Z",
		OriginalMaxAge: 23,
		HttpOnly:       true,
		Path:           "/",
		SameSite:       "Strict",
		Secure:         true,
	}

	memoryStore.Set(&sid, &sessionRetrieved)
	var retrievedSession = memoryStore.Get(sid)
	if retrievedSession.Expires != "2024-12-12T13:12:12+01:00" {
		t.Error("Expected", sessionRetrieved.Expires, "but got", retrievedSession.Expires)
	}
}

func TestSetOfSessionThatExpires(t *testing.T) {

	var nowPlus100 = time.Now().Add(100 * time.Millisecond).Format(time.RFC3339Nano)

	var memoryStore, sid = prepareTest()
	var sessionRetrieved = session.Session{
		Expires:        nowPlus100,
		OriginalMaxAge: 23,
		HttpOnly:       true,
		Path:           "/",
		SameSite:       "lax",
		Secure:         true,
	}
	memoryStore.Set(&sid, &sessionRetrieved)

	time.Sleep(110 * time.Millisecond)

	var retrievedSession = memoryStore.Get(sid)
	if retrievedSession != nil {
		t.Error("Expected nil but got", *retrievedSession)
	}
}

func TestSetOfAlreadyExpiredSession(t *testing.T) {
	var firstTickInUnix = time.UnixMicro(1).Format(time.RFC3339Nano)
	var sessionRetrieved = session.Session{
		Expires:        firstTickInUnix,
		OriginalMaxAge: 23,
		HttpOnly:       true,
		Path:           "/",
		SameSite:       "lax",
		Secure:         true,
	}

	var memoryStore, sid = prepareTest()
	memoryStore.Set(&sid, &sessionRetrieved)
	var retrievedSession = memoryStore.Get(sid)
	if retrievedSession != nil {
		t.Error("Expected nil but got", *retrievedSession)
	}
}

func TestSwitchFromNonExpiringToExpiring(t *testing.T) {
	var memoryStore, sid = prepareTest()
	var sess = session.Session{
		Secure:         false,
		SameSite:       "lax",
		Path:           "/",
		Expires:        "",
		HttpOnly:       true,
		OriginalMaxAge: 123,
	}

	memoryStore.Set(&sid, &sess)
	var nowPlus100 = time.Now().Add(110 * time.Millisecond).Format(time.RFC3339Nano)

	var retrievedSess = session.Session{
		Secure:         false,
		SameSite:       "lax",
		Path:           "/",
		Expires:        nowPlus100,
		HttpOnly:       true,
		OriginalMaxAge: 123,
	}
	memoryStore.Set(&sid, &retrievedSess)
	time.Sleep(110 * time.Millisecond)
	var retrievedSession = memoryStore.Get(sid)
	if retrievedSession != nil {
		t.Error("Expected nil but got", *retrievedSession)
	}
}
