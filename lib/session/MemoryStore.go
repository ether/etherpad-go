package session

// src/node/db/SessionStore.ts
import (
	"math"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/session"
	"github.com/gofiber/fiber/v2"
)

type Expiration struct {
	Timeout *time.Timer
	Db      *int64
	Real    *int64
}

type MemoryStore struct {
	db db.DataStore
	// Maps session ID to an object with the following properties:
	//   - `db`: Session expiration as recorded in the database (ms since epoch, not a Date).
	//   - `real`: Actual session expiration (ms since epoch, not a Date). Always greater than or
	//     equal to `db`.
	//   - `timeout`: Timeout ID for a timeout that will clean up the database record.
	expirations map[string]Expiration
	refresh     *int64
	generate    *func(c *fiber.Ctx)
}

func NewMemoryStore(db db.DataStore, refresh *int64) *MemoryStore {
	return &MemoryStore{
		db:          db,
		refresh:     refresh,
		expirations: make(map[string]Expiration),
	}
}

func generateMax(values ...int64) int64 {
	if len(values) == 0 {
		return math.MinInt64 // Return the smallest possible int64 value if no values are provided
	}

	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func (m *MemoryStore) ShutDown() {
	for _, val := range m.expirations {
		if val.Timeout != nil {
			val.Timeout.Stop()
		}
	}
}

func (m *MemoryStore) UpdateExpirations(sid *string, session *session.Session, updateDbExp *bool) *session.Session {

	if updateDbExp == nil {
		var truthy = true
		updateDbExp = &truthy
	}

	if sid == nil {
		return nil
	}

	var exp = m.expirations[*sid]
	if exp.Timeout != nil {
		exp.Timeout.Stop()
	}
	if session != nil && session.Expires != "" {
		layout := time.RFC3339
		parsedTime, err := time.Parse(layout, session.Expires)

		if err != nil {
			panic(err)
		}

		var sessExp = parsedTime.Unix()

		if updateDbExp != nil && *updateDbExp {
			exp.Db = &sessExp
		}

		var submittedReal int64
		var submittedDb int64
		if exp.Real != nil {
			submittedReal = *exp.Real
		} else {
			submittedReal = 0
		}

		if exp.Db != nil {
			submittedDb = *exp.Db
		} else {
			submittedDb = 0
		}

		var maxRetrieved = generateMax(submittedReal, submittedDb, sessExp)
		exp.Real = &maxRetrieved

		now := time.Now().Unix()

		if *exp.Real <= now {
			return m.Destroy(*sid)
		}

		// If reading from the database, update the expiration with the latest value from touch() so
		// that touch() appears to write to the database every time even though it doesn't.
		if session.Expires != "" {
			session.Expires = time.Unix(*exp.Real, 0).Format(layout)
		}
		// Use this._get(), not this._destroy(), to destroy the DB record for the expired session.
		// This is done in case multiple Etherpad instances are sharing the same database and users
		// are bouncing between the instances. By using this._get(), this instance will query the DB
		// for the latest expiration time written by any of the instances, ensuring that the record
		// isn't prematurely deleted if the expiration time was updated by a different Etherpad
		// instance. (Important caveat: Client-side database caching, which ueberdb does by default,
		// could still cause the record to be prematurely deleted because this instance might get a
		// stale expiration time from cache.)
		exp.Timeout = time.AfterFunc(time.Duration(*exp.Real-now), func() {
			m.Get(*sid)
		})
		m.expirations[*sid] = exp
	} else {
		delete(m.expirations, *sid)
	}
	return session
}

func (m *MemoryStore) Destroy(sid string) *session.Session {
	var retrievedExp, ok = m.expirations[sid]
	if ok {
		retrievedExp.Timeout.Stop()
	}
	foundSession, err := m.db.GetSessionById(sid)
	if err != nil {
		return nil
	}
	err = m.db.RemoveSessionById(sid)
	if err != nil {
		return nil
	}
	return foundSession
}

func (m *MemoryStore) Write(sid string, session session.Session) {
	m.db.SetSessionById(sid, session)
}

func (m *MemoryStore) Set(sid *string, session *session.Session) {
	sess := m.UpdateExpirations(sid, session, nil)
	if sess != nil {
		m.Write(*sid, *sess)
	}
}

func (m *MemoryStore) Shutdown() {
	for _, exp := range m.expirations {
		exp.Timeout.Stop()
	}
}

func (m *MemoryStore) Touch(sid string, session *session.Session) {
	var falsy = false
	var sess = m.UpdateExpirations(&sid, session, &falsy)

	if sess == nil {
		return
	}

	var exp, ok = m.expirations[sid]
	// If the session doesn't expire, don't do anything. Ideally we would write the session to the
	// database if it didn't already exist, but we have no way of knowing that without querying the
	// database. The query overhead is not worth it because set() should be called soon anyway.

	if !ok {
		return
	}

	if exp.Db != nil && (m.refresh == nil || *exp.Real < *exp.Db+*m.refresh) {
		return
	}
	m.Write(sid, *sess)
	layout := time.RFC3339
	parsedTime, err := time.Parse(layout, session.Expires)

	if err != nil {
		panic(err)
	}

	var pTime = parsedTime.Unix()
	exp.Db = &pTime
}

func (m *MemoryStore) Get(sid string) *session.Session {
	var retrievedSession, err = m.db.GetSessionById(sid)

	if err != nil {
		return nil
	}

	return m.UpdateExpirations(&sid, retrievedSession, nil)
}
