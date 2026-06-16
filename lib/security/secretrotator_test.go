package security

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// fakeStore is an in-memory SecretStore for deterministic tests.
type fakeStore struct {
	mu   sync.Mutex
	data map[string]fakeRow
}

type fakeRow struct {
	prefix  string
	payload string
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string]fakeRow{}} }

func (f *fakeStore) SaveSecretParams(id, prefix, payload string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[id] = fakeRow{prefix: prefix, payload: payload}
	return nil
}

func (f *fakeStore) ListSecretParams(prefix string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]string{}
	for id, row := range f.data {
		if row.prefix == prefix {
			out[id] = row.payload
		}
	}
	return out, nil
}

func (f *fakeStore) DeleteSecretParams(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, id)
	return nil
}

func (f *fakeStore) count(prefix string) int {
	m, _ := f.ListSecretParams(prefix)
	return len(m)
}

// newTestRotator builds a rotator wired to a manual clock and with the
// background timer disabled so update() can be stepped by hand.
func newTestRotator(store SecretStore, interval, lifetime time.Duration, now *int64) *SecretRotator {
	r := NewSecretRotator(store, "test", interval, lifetime, nil, nil)
	r.nowFn = func() int64 { return *now }
	r.stopped = true // prevent real timer scheduling in tests
	return r
}

func secretsContain(secrets [][]byte, target []byte) bool {
	for _, s := range secrets {
		if bytes.Equal(s, target) {
			return true
		}
	}
	return false
}

func TestSecretRotator_FirstRunGeneratesSecret(t *testing.T) {
	store := newFakeStore()
	now := int64(0)
	r := newTestRotator(store, time.Hour, time.Hour, &now)

	if err := r.update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	secrets := r.Secrets()
	if len(secrets) == 0 {
		t.Fatal("expected at least one secret")
	}
	if len(secrets[0]) < 32 {
		t.Fatalf("active secret too short for fosite (%d bytes)", len(secrets[0]))
	}
	if store.count("test") == 0 {
		t.Fatal("expected params persisted to store")
	}
}

func TestSecretRotator_StableWithinInterval(t *testing.T) {
	store := newFakeStore()
	now := int64(0)
	r := newTestRotator(store, time.Hour, time.Hour, &now)

	if err := r.update(); err != nil {
		t.Fatal(err)
	}
	first := r.Secrets()[0]

	// Re-running within the same interval must keep the active secret stable.
	now = int64(time.Minute.Milliseconds() * 10)
	if err := r.update(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, r.Secrets()[0]) {
		t.Fatal("active secret changed within the same interval")
	}
}

func TestSecretRotator_RotatesAndKeepsOldSecret(t *testing.T) {
	store := newFakeStore()
	now := int64(0)
	interval := time.Hour
	r := newTestRotator(store, interval, time.Hour, &now)

	if err := r.update(); err != nil {
		t.Fatal(err)
	}
	old := r.Secrets()[0]

	// Advance one full interval and rotate.
	now = interval.Milliseconds()
	if err := r.update(); err != nil {
		t.Fatal(err)
	}
	secrets := r.Secrets()

	if bytes.Equal(old, secrets[0]) {
		t.Fatal("active secret did not rotate after a full interval")
	}
	if !secretsContain(secrets, old) {
		t.Fatal("previous secret dropped from verification window after one interval")
	}
}

func TestSecretRotator_MultiInstanceSharesActiveSecret(t *testing.T) {
	store := newFakeStore()
	now := int64(0)

	a := newTestRotator(store, time.Hour, time.Hour, &now)
	if err := a.update(); err != nil {
		t.Fatal(err)
	}
	active := a.Secrets()[0]

	// A second instance starting against the same DB in the same interval must
	// adopt the already-published current secret rather than minting its own.
	b := newTestRotator(store, time.Hour, time.Hour, &now)
	if err := b.update(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(active, b.Secrets()[0]) {
		t.Fatal("second instance did not adopt the shared active secret")
	}
}

func TestSecretRotator_RemovesExpiredParams(t *testing.T) {
	store := newFakeStore()
	now := int64(0)
	interval := time.Hour
	lifetime := time.Hour
	r := newTestRotator(store, interval, lifetime, &now)

	// Seed a stale params set that ended long ago and is past end+lifetime+interval.
	iv := interval.Milliseconds()
	stale := secretParams{
		AlgID:     1,
		AlgParams: json.RawMessage(`{"digest":"sha256","keyLen":32,"salt":"00","secret":"00"}`),
		Start:     -10 * iv,
		End:       -9 * iv,
		Interval:  &iv,
		Lifetime:  lifetime.Milliseconds(),
	}
	payload, _ := json.Marshal(stale)
	if err := store.SaveSecretParams("stale", "test", string(payload)); err != nil {
		t.Fatal(err)
	}

	if err := r.update(); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.data["stale"]; ok {
		t.Fatal("expired params were not removed from the store")
	}
}

func TestSecretRotator_SkipsInvalidParams(t *testing.T) {
	store := newFakeStore()
	now := int64(0)
	r := newTestRotator(store, time.Hour, time.Hour, &now)

	// A row with an out-of-range algId and one with interval=0 would otherwise
	// panic (slice index / divide-by-zero). They must be skipped and removed.
	badAlg := `{"algId":99,"algParams":{},"start":0,"end":1,"interval":3600000,"lifetime":3600000}`
	zeroIv := `{"algId":1,"algParams":{"digest":"sha256","keyLen":32,"salt":"00","secret":"00"},"start":0,"end":1,"interval":0,"lifetime":3600000}`
	if err := store.SaveSecretParams("bad-alg", "test", badAlg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSecretParams("zero-iv", "test", zeroIv); err != nil {
		t.Fatal(err)
	}

	if err := r.update(); err != nil {
		t.Fatalf("update must not fail on invalid rows: %v", err)
	}
	if _, ok := store.data["bad-alg"]; ok {
		t.Error("invalid algId row was not removed")
	}
	if _, ok := store.data["zero-iv"]; ok {
		t.Error("zero-interval row was not removed")
	}
	// A fresh valid current secret must still have been generated.
	if len(r.Secrets()) == 0 {
		t.Error("expected a valid current secret despite invalid rows")
	}
}

func TestSecretRotator_SecretsAreDeepCopied(t *testing.T) {
	store := newFakeStore()
	now := int64(0)
	r := newTestRotator(store, time.Hour, time.Hour, &now)
	if err := r.update(); err != nil {
		t.Fatal(err)
	}

	snap := r.Secrets()
	if len(snap) == 0 || len(snap[0]) == 0 {
		t.Fatal("expected a non-empty active secret")
	}
	// Mutating the returned slice must not affect the rotator's internal secret.
	snap[0][0] ^= 0xFF
	if bytes.Equal(snap[0], r.Secrets()[0]) {
		t.Error("Secrets() returned an aliased slice; caller mutation leaked into rotator state")
	}
}

func TestSecretRotator_StartRejectsBadInterval(t *testing.T) {
	store := newFakeStore()
	r := NewSecretRotator(store, "test", 0, time.Hour, nil, nil)
	r.stopped = true
	if err := r.Start(); err == nil {
		t.Error("Start should reject a non-positive interval")
	}
}

func TestSecretRotator_LegacyStaticSecretIncluded(t *testing.T) {
	store := newFakeStore()
	now := int64(0)
	legacy := "this-is-a-legacy-static-secret-long-enough"
	r := NewSecretRotator(store, "test", time.Hour, time.Hour, &legacy, nil)
	r.nowFn = func() int64 { return now }
	r.stopped = true

	if err := r.update(); err != nil {
		t.Fatal(err)
	}
	if !secretsContain(r.Secrets(), []byte(legacy)) {
		t.Fatal("legacy static secret not present in active secrets")
	}
}
