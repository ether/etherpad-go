// Package versionstatus answers /api/version-status: whether to tell the
// visitor of a pad that this Etherpad is a minor version or more behind.
// Only the pad's first author is told, as the person most likely to run or
// know who runs the instance (etherpad-lite's updateStatus).
package versionstatus

import (
	"sort"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/updater"
	"github.com/gofiber/fiber/v3"
)

// Response is the answer; Outdated is "minor" or empty (null in JSON).
type Response struct {
	Outdated      *string `json:"outdated"`
	IsFirstAuthor bool    `json:"isFirstAuthor"`
}

var empty = Response{}

// Deps are what the handler reads.
type Deps struct {
	// Status is the updater's view: current and latest version.
	Status func() updater.Status
	// Pool returns the attribute pool of an existing pad, or false.
	Pool func(padID string) (*apool.APool, bool)
	// Author is the author id of the request's token cookie, or "".
	Author func(c fiber.Ctx) string
	// Now and TTL drive the cache; zero values mean time.Now and a minute.
	Now func() time.Time
	TTL time.Duration
	// Capacity bounds the cache (pad and author pairs); 0 means 1000.
	Capacity int
}

// FirstAuthor is the author of the lowest pool entry ['author', id] with an
// id, or "" for a pad nobody has written in yet.
func FirstAuthor(pool *apool.APool) string {
	if pool == nil {
		return ""
	}
	nums := make([]int, 0, len(pool.NumToAttrib))
	for n := range pool.NumToAttrib {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	for _, n := range nums {
		if a := pool.NumToAttrib[n]; a.Key == "author" && a.Value != "" {
			return a.Value
		}
	}
	return ""
}

type entry struct {
	value Response
	at    time.Time
}

// Handler is the /api/version-status handler. Answers are cached per pad
// and author for a minute: every pad load asks.
type Handler struct {
	d     Deps
	mu    sync.Mutex
	cache map[string]entry
	order []string // insertion order, for eviction
	// Computations counts cache misses (tests).
	Computations int
}

// New builds the handler.
func New(d Deps) *Handler {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.TTL == 0 {
		d.TTL = time.Minute
	}
	if d.Capacity == 0 {
		d.Capacity = 1000
	}
	return &Handler{d: d, cache: map[string]entry{}}
}

func (h *Handler) compute(padID, author string) Response {
	st := h.d.Status()
	if st.Latest == nil || !updater.MinorOrMoreBehind(st.CurrentVersion, st.Latest.Version) {
		return empty
	}
	if padID == "" || author == "" {
		return empty
	}
	pool, ok := h.d.Pool(padID)
	if !ok || FirstAuthor(pool) != author {
		return empty
	}
	minor := "minor"
	return Response{Outdated: &minor, IsFirstAuthor: true}
}

// Lookup answers for a pad and author, from the cache when it is fresh.
func (h *Handler) Lookup(padID, author string) Response {
	key := padID + "|" + author
	now := h.d.Now()
	h.mu.Lock()
	if e, ok := h.cache[key]; ok && now.Sub(e.at) <= h.d.TTL {
		h.mu.Unlock()
		return e.value
	}
	h.mu.Unlock()

	value := h.compute(padID, author)

	h.mu.Lock()
	defer h.mu.Unlock()
	h.Computations++
	if _, ok := h.cache[key]; !ok {
		h.order = append(h.order, key)
		for len(h.order) > h.d.Capacity {
			delete(h.cache, h.order[0])
			h.order = h.order[1:]
		}
	}
	h.cache[key] = entry{value: value, at: now}
	return value
}

// Serve is the fiber handler.
func (h *Handler) Serve(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return c.JSON(h.Lookup(c.Query("padId"), h.d.Author(c)))
}
