// Package metrics holds the Prometheus instruments on the hot path of the
// pad message handler: etherpad-lite's prom-instruments (scaling dive,
// #7756). They let a load test tell the apply path from the fan-out and
// see per-pad concurrency. Everything is off unless scalingDiveMetrics is
// set: the recorders are no-ops then and nothing is registered.
package metrics

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

var enabled atomic.Bool

// SetEnabled turns the instruments on or off (settings.scalingDiveMetrics).
func SetEnabled(on bool) { enabled.Store(on) }

// Enabled reports whether the instruments record.
func Enabled() bool { return enabled.Load() }

var (
	ChangesetApplyDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "etherpad_changeset_apply_duration_seconds",
		Help:    "Time spent applying an incoming USER_CHANGES message on the server (apply path only, excludes fan-out to other clients)",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5},
	})
	SocketEmitsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "etherpad_socket_emits_total",
		Help: "Number of socket broadcast emits, bucketed by message type",
	}, []string{"type"})
	PadUsers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "etherpad_pad_users",
		Help: "Active users connected to a pad, keyed by padId",
	}, []string{"padId"})
)

// knownTypes are the label values of SocketEmitsTotal; anything else (a
// plugin's or an HTTP API caller's message type) counts as "other", so the
// label set stays bounded.
var knownTypes = map[string]bool{
	"NEW_CHANGES": true, "ACCEPT_COMMIT": true, "CHAT_MESSAGE": true, "CLIENT_VARS": true,
	"CLIENT_MESSAGE": true, "CUSTOM": true, "USER_NEWINFO": true, "USERINFO_UPDATE": true, "USER_LEAVE": true,
}

// RecordSocketEmit counts one emit of a message type.
func RecordSocketEmit(msgType string) {
	if !Enabled() {
		return
	}
	if !knownTypes[msgType] {
		msgType = "other"
	}
	SocketEmitsTotal.WithLabelValues(msgType).Inc()
}

// RecordChangesetApply starts timing the apply path; call the result once
// the revision is stored. A failed apply leaves it uncalled, so the
// distribution is of successful applies only.
func RecordChangesetApply() func() {
	if !Enabled() {
		return func() {}
	}
	timer := prometheus.NewTimer(ChangesetApplyDuration)
	return func() { timer.ObserveDuration() }
}

// SetPadUsers replaces the per-pad user counts, so drained pads leave no
// stale series.
func SetPadUsers(counts map[string]int) {
	if !Enabled() {
		return
	}
	PadUsers.Reset()
	for padID, n := range counts {
		PadUsers.WithLabelValues(padID).Set(float64(n))
	}
}

// Register adds the instruments to reg when they are enabled.
func Register(reg prometheus.Registerer) {
	if !Enabled() {
		return
	}
	reg.MustRegister(ChangesetApplyDuration, SocketEmitsTotal, PadUsers)
}
