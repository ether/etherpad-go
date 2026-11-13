package security

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/hkdf"
)

// Port of etherpad-lite's SecretRotator spec. fakeClock plays the role of
// upstream's FakeClock: rotations run when the test advances the clock.

type fakeTimeout struct {
	id   int
	when int64
	fn   func()
}

type fakeClock struct {
	now      int64
	nextID   int
	timeouts map[int]*fakeTimeout
}

func newFakeClock() *fakeClock { return &fakeClock{timeouts: map[int]*fakeTimeout{}} }

func (c *fakeClock) schedule(delay int64, fn func()) func() {
	c.nextID++
	id := c.nextID
	c.timeouts[id] = &fakeTimeout{id: id, when: c.now + delay, fn: fn}
	return func() { delete(c.timeouts, id) }
}

func (c *fakeClock) next() int64 {
	n := int64(math.MaxInt64)
	for _, to := range c.timeouts {
		n = min(n, to.when)
	}
	return n
}

// setNow moves the clock to t, running every timeout due on the way, each at
// its own time.
func (c *fakeClock) setNow(t *testing.T, to int64) {
	t.Helper()
	require.GreaterOrEqual(t, to, c.now)
	for {
		n := c.next()
		if n > to {
			break
		}
		c.now = max(c.now, n)
		var due []*fakeTimeout
		for _, x := range c.timeouts {
			if x.when <= c.now {
				due = append(due, x)
			}
		}
		sort.Slice(due, func(i, j int) bool {
			return due[i].when < due[j].when || (due[i].when == due[j].when && due[i].id < due[j].id)
		})
		for _, x := range due {
			delete(c.timeouts, x.id)
			x.fn()
		}
	}
	c.now = to
}

func (c *fakeClock) advance(t *testing.T, d int64) { c.setNow(t, c.now+d) }

func (c *fakeClock) advanceToNext(t *testing.T) {
	if n := c.next(); n < math.MaxInt64 {
		c.setNow(t, max(n, c.now))
	}
}

const parityLifetime = int64(1e4)

type parityEnv struct {
	store    *fakeStore
	prefix   string
	interval int64
}

func newParityEnv() *parityEnv {
	return &parityEnv{store: newFakeStore(), prefix: "test-SecretRotator", interval: 1e3}
}

func (e *parityEnv) rotator(fc *fakeClock, legacy ...string) *SecretRotator {
	var l *string
	if len(legacy) > 0 {
		l = &legacy[0]
	}
	r := NewSecretRotator(e.store, e.prefix, time.Duration(e.interval)*time.Millisecond, time.Duration(parityLifetime)*time.Millisecond, l, nil)
	r.nowFn = func() int64 { return fc.now }
	r.schedule = fc.schedule
	return r
}

func (e *parityEnv) params(t *testing.T) map[string]secretParams {
	t.Helper()
	rows, err := e.store.ListSecretParams(e.prefix)
	require.NoError(t, err)
	out := map[string]secretParams{}
	for id, payload := range rows {
		var p secretParams
		require.NoError(t, json.Unmarshal([]byte(payload), &p))
		out[id] = p
	}
	return out
}

func (e *parityEnv) ids(t *testing.T) []string {
	ids := make([]string, 0)
	for id := range e.params(t) {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func strs(secrets [][]byte) []string {
	out := make([]string, len(secrets))
	for i, s := range secrets {
		out[i] = string(s)
	}
	return out
}

// hkdfHex is upstream's hkdf(secret, salt, tN): Node's crypto.hkdf with the
// hex strings as key material, 32 bytes, hex-encoded.
func hkdfHex(t *testing.T, secret, salt string, tN int64) string {
	r := hkdf.New(sha256.New, []byte(secret), []byte(salt), []byte(strconv.FormatInt(tN, 10)))
	out := make([]byte, 32)
	_, err := io.ReadFull(r, out)
	require.NoError(t, err)
	return hex.EncodeToString(out)
}

func intervalStartOf(t, iv int64) int64 { return t - mod(t, iv) }

func TestSecretRotatorParityConstructor(t *testing.T) {
	r := newParityEnv().rotator(newFakeClock())
	assert.Empty(t, r.Secrets(), "creates an empty secrets array")
}

func TestSecretRotatorParityStart(t *testing.T) {
	t.Run("derives secrets", func(t *testing.T) {
		e := newParityEnv()
		r := e.rotator(newFakeClock())
		require.NoError(t, r.Start())
		defer r.Stop()
		s := strs(r.Secrets())
		require.Len(t, s, 3, "current, previous and next")
		for _, x := range s {
			assert.NotEmpty(t, x)
		}
		assert.Len(t, slices.Compact(slices.Sorted(slices.Values(s))), 3, "the secrets all differ")
	})

	t.Run("publishes params", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		params := e.params(t)
		require.Len(t, params, 1)
		for id, p := range params {
			assert.NotEmpty(t, id)
			var kdf kdfParams
			require.NoError(t, json.Unmarshal(p.AlgParams, &kdf))
			assert.Equal(t, 1, p.AlgID)
			assert.Equal(t, "sha256", kdf.Digest)
			assert.Equal(t, 32, kdf.KeyLen)
			assert.Regexp(t, `^[0-9a-f]{64}$`, kdf.Salt)
			assert.Regexp(t, `^[0-9a-f]{64}$`, kdf.Secret)
			assert.Equal(t, fc.now, p.Start)
			assert.Equal(t, fc.now+2*e.interval, p.End)
			require.NotNil(t, p.Interval)
			assert.Equal(t, e.interval, *p.Interval)
			assert.Equal(t, parityLifetime, p.Lifetime)
			// Same derivation as etherpad-lite, in the same order.
			want := []string{
				hkdfHex(t, kdf.Secret, kdf.Salt, 0),
				hkdfHex(t, kdf.Secret, kdf.Salt, -e.interval),
				hkdfHex(t, kdf.Secret, kdf.Salt, e.interval),
			}
			assert.Equal(t, want, strs(r.Secrets()))
		}
	})

	t.Run("reuses a matching publication if unexpired", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		secrets, ids := strs(r.Secrets()), e.ids(t)
		r.Stop()
		r = e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		assert.Equal(t, secrets, strs(r.Secrets()))
		assert.Equal(t, ids, e.ids(t))
	})

	t.Run("deletes expired publications", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		oldID := e.ids(t)[0]
		p := e.params(t)[oldID]
		r.Stop()
		fc.setNow(t, p.End+p.Lifetime+*p.Interval)
		r = e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		ids := e.ids(t)
		require.Len(t, ids, 1)
		assert.NotEqual(t, oldID, ids[0])
	})

	t.Run("keeps expired publications until an interval past expiration", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		future := strs(r.Secrets())[2]
		r.Stop()
		origID := e.ids(t)[0]
		p := e.params(t)[origID]
		fc.advance(t, p.End+p.Lifetime+*p.Interval-1)
		r = e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		assert.Contains(t, strs(r.Secrets())[1:], future)
		assert.Len(t, e.ids(t), 2, "a new publication, not an extended old one")
		assert.Equal(t, p, e.params(t)[origID])
	})

	t.Run("idempotent", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		assert.Len(t, fc.timeouts, 1)
		secrets, ids := strs(r.Secrets()), e.ids(t)
		require.NoError(t, r.Start())
		assert.Len(t, fc.timeouts, 1, "starting again leaves one scheduled rotation")
		assert.Equal(t, secrets, strs(r.Secrets()))
		assert.Equal(t, ids, e.ids(t))
	})

	for _, tc := range []struct{ now, want int64 }{{0, 1e3}, {1, 1e3}, {999, 1e3}, {1e3, 2e3}, {1001, 2e3}} {
		t.Run("schedules the update at the next interval: "+strconv.FormatInt(tc.now, 10), func(t *testing.T) {
			e := newParityEnv()
			fc := newFakeClock()
			fc.setNow(t, tc.now)
			r := e.rotator(fc)
			require.NoError(t, r.Start())
			defer r.Stop()
			require.Len(t, fc.timeouts, 1)
			for _, to := range fc.timeouts {
				assert.Equal(t, tc.want, to.when)
			}
		})
	}

	t.Run("multiple active params with different intervals", func(t *testing.T) {
		e := newParityEnv()
		intervals := []int64{400, 600, 1000}
		wants := map[int64]bool{}
		for _, iv := range intervals {
			for x := iv; x <= 6000; x += iv { // lcm(400, 600, 1000) = 6000
				wants[x] = true
			}
		}
		shared := newFakeClock()
		var others []*SecretRotator
		for _, iv := range intervals {
			e.interval = iv
			r := e.rotator(shared)
			require.NoError(t, r.Start())
			others = append(others, r)
		}
		defer func() {
			for _, r := range others {
				r.Stop()
			}
		}()
		e.interval = intervals[len(intervals)-1]
		fc := newFakeClock() // a single instance's own clock
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		for _, want := range slices.Sorted(func(yield func(int64) bool) {
			for k := range wants {
				if !yield(k) {
					return
				}
			}
		}) {
			fc.advanceToNext(t)
			shared.setNow(t, fc.now) // keeps all publications alive
			assert.Equal(t, want, fc.now)
		}
	})
}

func TestSecretRotatorParityStop(t *testing.T) {
	e := newParityEnv()
	fc := newFakeClock()
	r := e.rotator(fc)
	require.NoError(t, r.Start())
	assert.NotEmpty(t, fc.timeouts)
	r.Stop()
	assert.Empty(t, fc.timeouts, "clears the timeout")
	r.Stop() // safe to call twice
}

func TestSecretRotatorParityLegacy(t *testing.T) {
	t.Run("ends at now if there are no previously published secrets", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		fc.setNow(t, 1)
		r := e.rotator(fc, "legacy")
		require.NoError(t, r.Start())
		defer r.Stop()
		s := strs(r.Secrets())
		require.Len(t, s, 4, "legacy plus past, current, future")
		assert.Contains(t, s[1:], "legacy", "not the current secret")
		var legacy, rotated []secretParams
		for _, p := range e.params(t) {
			if p.AlgID == 0 {
				legacy = append(legacy, p)
			} else {
				rotated = append(rotated, p)
			}
		}
		require.Len(t, legacy, 1)
		require.Len(t, rotated, 1)
		assert.JSONEq(t, `"legacy"`, string(legacy[0].AlgParams))
		assert.Equal(t, fc.now, legacy[0].Start)
		assert.Equal(t, fc.now, legacy[0].End, "start equals end so other instances' legacy end is unaffected")
		assert.Nil(t, legacy[0].Interval)
		assert.Equal(t, parityLifetime, legacy[0].Lifetime)
		assert.Equal(t, fc.now, rotated[0].Start)
		assert.Equal(t, intervalStartOf(fc.now, e.interval)+2*e.interval, rotated[0].End)
	})

	t.Run("ends at the start of the oldest previously published secret", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		fc.setNow(t, 1)
		wantTime := fc.now
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		s := strs(r.Secrets())
		require.Len(t, s, 3)
		s1, s0, s2 := s[0], s[1], s[2]
		r.Stop()
		fc.advance(t, e.interval+1)
		r = e.rotator(fc, "legacy")
		require.NoError(t, r.Start())
		defer r.Stop()
		got := strs(r.Secrets())
		require.Len(t, got, 5)
		assert.Equal(t, []string{s2, s1, s0, got[3], "legacy"}, got)
		for _, p := range e.params(t) {
			if p.AlgID == 0 {
				assert.Equal(t, wantTime, p.Start)
				assert.Equal(t, wantTime, p.End)
			} else {
				assert.Equal(t, wantTime, p.Start)
				assert.Equal(t, intervalStartOf(fc.now, e.interval)+2*e.interval, p.End)
			}
		}
	})

	t.Run("multiple instances with different legacy secrets", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc, "legacy1")
		require.NoError(t, r.Start())
		r.Stop()
		r = e.rotator(fc, "legacy2")
		require.NoError(t, r.Start())
		defer r.Stop()
		s := strs(r.Secrets())[1:]
		assert.Contains(t, s, "legacy1")
		assert.Contains(t, s, "legacy2")
	})

	t.Run("multiple instances with the same legacy secret", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc, "legacy")
		require.NoError(t, r.Start())
		r.Stop()
		r = e.rotator(fc, "legacy")
		require.NoError(t, r.Start())
		defer r.Stop()
		s := strs(r.Secrets())
		assert.Len(t, slices.Compact(slices.Sorted(slices.Values(s))), len(s), "no duplicates")
		assert.Len(t, e.ids(t), 2, "one publication for the legacy secret")
	})

	t.Run("included for an interval after expiration", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		r.Stop()
		fc.advance(t, parityLifetime+e.interval-1)
		r = e.rotator(fc, "legacy")
		require.NoError(t, r.Start())
		defer r.Stop()
		assert.Contains(t, strs(r.Secrets())[1:], "legacy")
	})

	t.Run("not included if the oldest secret is old enough", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		r.Stop()
		fc.advance(t, parityLifetime+e.interval)
		r = e.rotator(fc, "legacy")
		require.NoError(t, r.Start())
		defer r.Stop()
		assert.NotContains(t, strs(r.Secrets()), "legacy")
	})

	t.Run("dead secrets still affect the legacy end time", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		old := strs(r.Secrets())
		r.Stop()
		fc.advance(t, parityLifetime+3*e.interval)
		r = e.rotator(fc, "legacy")
		require.NoError(t, r.Start())
		defer r.Stop()
		s := strs(r.Secrets())
		assert.NotContains(t, s, "legacy")
		for _, x := range old {
			assert.NotContains(t, s, x)
		}
	})
}

func TestSecretRotatorParityRotation(t *testing.T) {
	t.Run("no rotation before the start of an interval", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		before := strs(r.Secrets())
		fc.advance(t, e.interval-1)
		assert.Equal(t, before, strs(r.Secrets()))
	})

	t.Run("the future secret becomes current, a new future one is generated", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		s := strs(r.Secrets())
		require.Len(t, s, 3)
		s1, s0, s2 := s[0], s[1], s[2]
		fc.advance(t, e.interval)
		got := strs(r.Secrets())
		require.Len(t, got, 4)
		assert.Equal(t, []string{s2, s1, s0, got[3]}, got)
		assert.NotContains(t, s, got[3])
	})

	t.Run("expired publications are deleted", func(t *testing.T) {
		e := newParityEnv()
		origInterval := e.interval
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		r.Stop()
		e.interval++ // forces new params so the old ones can expire
		r = e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		assert.Len(t, e.ids(t), 2)
		fc.advance(t, parityLifetime+3*origInterval)
		assert.Len(t, e.ids(t), 1)
	})

	t.Run("old secrets are eventually removed", func(t *testing.T) {
		e := newParityEnv()
		fc := newFakeClock()
		r := e.rotator(fc)
		require.NoError(t, r.Start())
		defer r.Stop()
		s0 := strs(r.Secrets())[1]
		fc.advance(t, parityLifetime+e.interval-1)
		assert.Contains(t, strs(r.Secrets())[1:], s0)
		fc.advance(t, 1)
		assert.NotContains(t, strs(r.Secrets()), s0)
	})
}

func TestSecretRotatorParityClockSkew(t *testing.T) {
	contains := func(secrets [][]byte, s []byte) bool { return secretsContain(secrets, s) }

	t.Run("out of sync works if in the adjacent interval", func(t *testing.T) {
		e := newParityEnv()
		fcs := []*fakeClock{newFakeClock(), newFakeClock()}
		srs := []*SecretRotator{e.rotator(fcs[0]), e.rotator(fcs[1])}
		for _, r := range srs {
			require.NoError(t, r.Start())
			defer r.Stop()
		}
		assert.Equal(t, strs(srs[0].Secrets()), strs(srs[1].Secrets()))
		check := func() {
			assert.True(t, contains(srs[0].Secrets(), srs[1].Secrets()[0]))
			assert.True(t, contains(srs[1].Secrets(), srs[0].Secrets()[0]))
		}
		fcs[0].advance(t, 2*e.interval-1)
		check()
		fcs[1].advance(t, e.interval)
		fcs[0].advance(t, e.interval)
		check()
		fcs[1].advance(t, 3*e.interval-1)
		fcs[0].advance(t, 1)
		check()
	})

	t.Run("start up out of sync", func(t *testing.T) {
		e := newParityEnv()
		fcs := []*fakeClock{newFakeClock(), newFakeClock()}
		srs := []*SecretRotator{e.rotator(fcs[0]), e.rotator(fcs[1])}
		fcs[0].advance(t, 2*e.interval-1)
		require.NoError(t, srs[0].Start())
		defer srs[0].Stop()
		require.NoError(t, srs[1].Start())
		defer srs[1].Stop()
		assert.True(t, contains(srs[0].Secrets(), srs[1].Secrets()[0]))
		assert.True(t, contains(srs[1].Secrets(), srs[0].Secrets()[0]))
	})
}
