// Package security provides the SecretRotator, a port of Etherpad's
// src/node/security/SecretRotator.ts. It maintains an array of secrets across
// one or more Etherpad instances sharing the same database, periodically
// rotating in a new secret and removing the oldest secret.
//
// The secrets are generated using a key derivation function (HKDF) with input
// keying material coming from a long-lived secret stored in the database
// (generated if missing). The first secret in the array is the one that should
// be used to generate new MACs/signatures; every secret in the array should be
// accepted when validating an existing MAC/signature.
package security

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/hkdf"
)

// SecretStore is the persistence contract backing the rotator. *db.DataStore
// satisfies it through the dedicated secret_rotation table.
type SecretStore interface {
	SaveSecretParams(id string, prefix string, payload string) error
	ListSecretParams(prefix string) (map[string]string, error)
	DeleteSecretParams(id string) error
}

// kdfParams holds the persisted parameters for the HKDF algorithm. Secret and
// salt are hex-encoded random bytes.
type kdfParams struct {
	Digest string `json:"digest"`
	KeyLen int    `json:"keyLen"`
	Salt   string `json:"salt"`
	Secret string `json:"secret"`
}

// algorithm is a key derivation algorithm. Entries in the algorithms slice are
// addressed by index (algId) which is persisted, so the slice must only ever be
// appended to (mirrors the upstream comment in SecretRotator.ts).
type algorithm interface {
	generateParams() (json.RawMessage, error)
	derive(algParams json.RawMessage, info string) ([]byte, error)
}

// legacyStaticSecret (algId 0) returns a fixed secret regardless of info. Its
// algParams is a JSON string holding the secret itself. Used to migrate an
// existing static secret into the rotation scheme.
type legacyStaticSecret struct{}

func (legacyStaticSecret) generateParams() (json.RawMessage, error) {
	return nil, errors.New("legacyStaticSecret cannot generate params")
}

func (legacyStaticSecret) derive(algParams json.RawMessage, _ string) ([]byte, error) {
	var s string
	if err := json.Unmarshal(algParams, &s); err != nil {
		return nil, err
	}
	return []byte(s), nil
}

// hkdfAlgorithm (algId 1) derives per-interval secrets from a stored random
// secret using HKDF, salted per-params and with the interval start time as the
// info parameter.
type hkdfAlgorithm struct {
	keyLen int
}

func (h hkdfAlgorithm) generateParams() (json.RawMessage, error) {
	secret := make([]byte, h.keyLen)
	salt := make([]byte, h.keyLen)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return json.Marshal(kdfParams{
		Digest: "sha256",
		KeyLen: h.keyLen,
		Salt:   hex.EncodeToString(salt),
		Secret: hex.EncodeToString(secret),
	})
}

func (h hkdfAlgorithm) derive(algParams json.RawMessage, info string) ([]byte, error) {
	var p kdfParams
	if err := json.Unmarshal(algParams, &p); err != nil {
		return nil, err
	}
	if p.KeyLen <= 0 || p.KeyLen > 1024 {
		return nil, fmt.Errorf("invalid hkdf keyLen %d", p.KeyLen)
	}
	// secret/salt are hex strings; their literal bytes are used as IKM and salt.
	// Internal consistency is all that matters (Go-only deployment), so the
	// exact byte interpretation need not match Node's crypto.hkdf.
	r := hkdf.New(sha256.New, []byte(p.Secret), []byte(p.Salt), []byte(info))
	out := make([]byte, p.KeyLen)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	// Hex-encode so the active secret is comfortably longer than fosite's 32-byte
	// minimum and is safe to log/store as text.
	dst := make([]byte, hex.EncodedLen(len(out)))
	hex.Encode(dst, out)
	return dst, nil
}

// algorithms is append-only. defaultAlgID always points at the last entry.
var algorithms = []algorithm{
	legacyStaticSecret{},
	hkdfAlgorithm{keyLen: 32},
}

var defaultAlgID = len(algorithms) - 1

// secretParams is one published parameter set. interval is nil for the legacy
// static secret. All times are unix milliseconds.
type secretParams struct {
	AlgID     int             `json:"algId"`
	AlgParams json.RawMessage `json:"algParams"`
	Start     int64           `json:"start"`
	End       int64           `json:"end"`
	Interval  *int64          `json:"interval"`
	Lifetime  int64           `json:"lifetime"`
}

// SecretRotator maintains a rotating array of secrets persisted in the database.
type SecretRotator struct {
	mu       sync.RWMutex
	secrets  [][]byte
	prefix   string
	interval int64 // ms
	lifetime int64 // ms
	legacy   *string

	store    SecretStore
	logger   *zap.SugaredLogger
	nowFn    func() int64 // unix ms; injectable for tests
	onRotate func()       // optional, invoked after every successful update

	timer   *time.Timer
	stopped bool
}

// NewSecretRotator creates a rotator. interval is how often a new secret is
// rotated in; lifetime is how long after the end of an interval a secret stays
// usable. legacyStaticSecret, if non-nil, covers the period before the first
// rotated secret to ease migration from a previously static secret.
func NewSecretRotator(store SecretStore, prefix string, interval, lifetime time.Duration, legacyStaticSecret *string, logger *zap.SugaredLogger) *SecretRotator {
	return &SecretRotator{
		prefix:   prefix,
		interval: interval.Milliseconds(),
		lifetime: lifetime.Milliseconds(),
		legacy:   legacyStaticSecret,
		store:    store,
		logger:   logger,
		nowFn:    func() int64 { return time.Now().UnixMilli() },
	}
}

// OnRotate registers a callback invoked after every successful update (initial
// and each rotation). Must be set before Start.
func (r *SecretRotator) OnRotate(fn func()) { r.onRotate = fn }

// Secrets returns a snapshot of the active secrets. The first entry is the one
// to use for generating new MACs/signatures; all entries are valid for
// verification.
func (r *SecretRotator) Secrets() [][]byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([][]byte, len(r.secrets))
	for i, s := range r.secrets {
		// Deep-copy each secret so callers cannot mutate the rotator's internal
		// signing/verification material.
		out[i] = bytes.Clone(s)
	}
	return out
}

// Start performs the first update synchronously (so Secrets is populated before
// use) and schedules subsequent rotations.
func (r *SecretRotator) Start() error {
	if r.interval <= 0 {
		return fmt.Errorf("secret rotator interval must be positive, got %d ms", r.interval)
	}
	if r.lifetime < 0 {
		return fmt.Errorf("secret rotator lifetime must not be negative, got %d ms", r.lifetime)
	}
	return r.update()
}

// Stop cancels the scheduled rotation.
func (r *SecretRotator) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

func mod(a, n int64) int64 { return ((a % n) + n) % n }

func intervalStart(t, interval int64) int64 { return t - mod(t, interval) }

// deriveSecrets derives all relevant secrets for one parameter set as of now.
func (r *SecretRotator) deriveSecrets(p secretParams, now int64) ([][]byte, error) {
	alg := algorithms[p.AlgID]
	if p.Interval == nil {
		s, err := alg.derive(p.AlgParams, "")
		if err != nil {
			return nil, err
		}
		return [][]byte{s}, nil
	}
	iv := *p.Interval
	t0 := intervalStart(now, iv)
	// Start of the first interval covered by these params, backdated by iv to
	// accommodate clock skew between instances.
	tA := intervalStart(p.Start-iv, iv)
	tZ := intervalStart(p.End-1, iv)
	expired := func(tN int64) bool { return now >= tN+(2*iv)+p.Lifetime }

	start := min(t0, tZ)
	var tNs []int64
	// Walk back from start to the start of coverage; t0 must be first so its
	// derived secret (used to generate new MACs) is secrets[0].
	for tN := start; tN >= tA && !expired(tN); tN -= iv {
		tNs = append(tNs, tN)
	}
	// Include a future derived secret to accommodate clock skew.
	if t0+iv <= tZ {
		tNs = append(tNs, t0+iv)
	}
	out := make([][]byte, 0, len(tNs))
	for _, tN := range tNs {
		s, err := alg.derive(p.AlgParams, strconv.FormatInt(tN, 10))
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// publish writes a parameter set to the database. When id is empty a random id
// is generated to avoid races with other instances.
func (r *SecretRotator) publish(p secretParams, id string) (string, error) {
	if id == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		id = hex.EncodeToString(buf)
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	if err := r.store.SaveSecretParams(id, r.prefix, string(payload)); err != nil {
		return "", err
	}
	return id, nil
}

func marshalString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// update reconciles the published parameter sets with the database, derives the
// active secrets, and schedules the next rotation. It mirrors
// SecretRotator.ts#_update.
func (r *SecretRotator) update() error {
	now := r.nowFn()
	t0 := intervalStart(now, r.interval)
	next := t0 + r.interval
	legacyEnd := now

	dbMap, err := r.store.ListSecretParams(r.prefix)
	if err != nil {
		return err
	}

	var currentParams *secretParams
	currentID := ""
	var allParams []secretParams
	var legacyParams []secretParams

	for id, payload := range dbMap {
		var p secretParams
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			if r.logger != nil {
				r.logger.Warnf("secret-rotation %s: dropping unparseable params %s: %v", r.prefix, id, err)
			}
			continue
		}
		// Defend against corrupted/tampered rows: an out-of-range algId would
		// index past `algorithms`, and a non-positive interval would divide by
		// zero in intervalStart(). Skip such rows so a bad row can never crash a
		// scheduled rotation.
		if !validParams(p) {
			if r.logger != nil {
				r.logger.Warnf("secret-rotation %s: dropping invalid params %s (algId=%d)", r.prefix, id, p.AlgID)
			}
			if err := r.store.DeleteSecretParams(id); err != nil && r.logger != nil {
				r.logger.Warnf("secret-rotation %s: failed to remove invalid params %s: %v", r.prefix, id, err)
			}
			continue
		}
		if p.AlgID == 0 && r.legacy != nil {
			var s string
			if err := json.Unmarshal(p.AlgParams, &s); err == nil && s == *r.legacy {
				legacyParams = append(legacyParams, p)
			}
		}
		if p.Start < legacyEnd {
			legacyEnd = p.Start
		}

		pInterval := r.interval
		if p.Interval != nil {
			pInterval = *p.Interval
		}
		// Expired: a MAC derived from these params can no longer be valid.
		if now >= p.End+p.Lifetime+pInterval {
			if err := r.store.DeleteSecretParams(id); err != nil && r.logger != nil {
				r.logger.Warnf("secret-rotation %s: failed to remove expired params %s: %v", r.prefix, id, err)
			}
			continue
		}

		hasInterval := p.Interval != nil
		var t1 int64
		if hasInterval {
			t1 = intervalStart(now, *p.Interval) + *p.Interval
			if t1 < next {
				next = t1
			}
		}
		if hasInterval {
			tA := intervalStart(p.Start, *p.Interval)
			if *p.Interval == r.interval && p.Lifetime == r.lifetime &&
				tA <= t1 && p.End > now &&
				(currentParams == nil || p.Start > currentParams.Start) {
				if currentParams != nil {
					allParams = append(allParams, *currentParams)
				}
				cp := p
				currentParams = &cp
				currentID = id
				continue
			}
		}
		allParams = append(allParams, p)
	}

	// Cover the gap before the first rotated secret with the legacy static
	// secret, if one was supplied and nothing already covers it.
	if r.legacy != nil && now < legacyEnd+r.lifetime+r.interval &&
		!legacyCovered(legacyParams, legacyEnd, r.lifetime) {
		p := secretParams{
			AlgID:     0,
			AlgParams: marshalString(*r.legacy),
			Start:     legacyEnd,
			End:       legacyEnd,
			Interval:  nil,
			Lifetime:  r.lifetime,
		}
		allParams = append(allParams, p)
		if _, err := r.publish(p, ""); err != nil && r.logger != nil {
			r.logger.Warnf("secret-rotation %s: failed to publish legacy params: %v", r.prefix, err)
		}
	}

	if currentParams == nil {
		algParams, err := algorithms[defaultAlgID].generateParams()
		if err != nil {
			return err
		}
		iv := r.interval
		currentParams = &secretParams{
			AlgID:     defaultAlgID,
			AlgParams: algParams,
			Start:     now,
			End:       now, // extended below
			Interval:  &iv,
			Lifetime:  r.lifetime,
		}
	}
	// Advance expiration to the end of the next interval so params never expire
	// under normal operation. Must happen before deriving secrets.
	currentParams.End = max(currentParams.End, t0+(2*r.interval))
	if _, err := r.publish(*currentParams, currentID); err != nil && r.logger != nil {
		r.logger.Warnf("secret-rotation %s: failed to publish current params: %v", r.prefix, err)
	}

	// Secrets derived from currentParams MUST come first.
	secrets, err := r.deriveSecrets(*currentParams, now)
	if err != nil {
		return err
	}
	for _, p := range allParams {
		s, err := r.deriveSecrets(p, now)
		if err != nil {
			return err
		}
		secrets = append(secrets, s...)
	}

	r.mu.Lock()
	r.secrets = secrets
	stopped := r.stopped
	if !stopped {
		delay := max(next-r.nowFn(), 0)
		r.timer = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
			if err := r.update(); err != nil && r.logger != nil {
				r.logger.Warnf("secret-rotation %s: scheduled update failed: %v", r.prefix, err)
			}
		})
	}
	r.mu.Unlock()

	if r.logger != nil {
		r.logger.Debugf("secret-rotation %s: %d active secret(s)", r.prefix, len(secrets))
	}
	if r.onRotate != nil {
		r.onRotate()
	}
	return nil
}

// validParams reports whether a persisted parameter set is safe to use. It
// guards the two values consumed without further checks downstream: AlgID
// (indexes the algorithms slice) and Interval (used as a modulo divisor).
func validParams(p secretParams) bool {
	if p.AlgID < 0 || p.AlgID >= len(algorithms) {
		return false
	}
	if p.Interval != nil && *p.Interval <= 0 {
		return false
	}
	if p.Lifetime < 0 {
		return false
	}
	return true
}

// legacyCovered reports whether an existing legacy params set already covers the
// period ending at legacyEnd+lifetime.
func legacyCovered(legacyParams []secretParams, legacyEnd, lifetime int64) bool {
	for _, p := range legacyParams {
		if p.End+p.Lifetime >= legacyEnd+lifetime {
			return true
		}
	}
	return false
}
