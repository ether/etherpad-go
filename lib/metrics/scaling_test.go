package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's prom-instruments spec.

func reset(t *testing.T, on bool) {
	SocketEmitsTotal.Reset()
	PadUsers.Reset()
	ChangesetApplyDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "etherpad_changeset_apply_duration_seconds", Help: "test",
	})
	SetEnabled(on)
	t.Cleanup(func() { SetEnabled(false) })
}

func applyCount(t *testing.T) uint64 {
	m := &dto.Metric{}
	require.NoError(t, ChangesetApplyDuration.Write(m))
	return m.GetHistogram().GetSampleCount()
}

func TestRecordSocketEmit(t *testing.T) {
	reset(t, true)
	RecordSocketEmit("NEW_CHANGES")
	RecordSocketEmit("NEW_CHANGES")
	RecordSocketEmit("CHAT_MESSAGE")
	assert.Equal(t, 2.0, testutil.ToFloat64(SocketEmitsTotal.WithLabelValues("NEW_CHANGES")))
	assert.Equal(t, 1.0, testutil.ToFloat64(SocketEmitsTotal.WithLabelValues("CHAT_MESSAGE")))
}

func TestRecordSocketEmitBoundsLabels(t *testing.T) {
	reset(t, true)
	RecordSocketEmit("")
	RecordSocketEmit("attacker-supplied-string-1")
	RecordSocketEmit("attacker-supplied-string-2")
	assert.Equal(t, 3.0, testutil.ToFloat64(SocketEmitsTotal.WithLabelValues("other")))
	assert.Equal(t, 1, testutil.CollectAndCount(SocketEmitsTotal), "only the other series")
}

func TestRecordChangesetApply(t *testing.T) {
	reset(t, true)
	end := RecordChangesetApply()
	time.Sleep(5 * time.Millisecond)
	end()
	assert.Equal(t, uint64(1), applyCount(t))
}

func TestSetPadUsers(t *testing.T) {
	reset(t, true)
	SetPadUsers(map[string]int{"a": 2, "b": 1})
	SetPadUsers(map[string]int{"a": 3})
	assert.Equal(t, 3.0, testutil.ToFloat64(PadUsers.WithLabelValues(PadLabel("a"))))
	assert.Equal(t, 1, testutil.CollectAndCount(PadUsers), "the drained pad b is gone")
}

// /metrics is public: the series carry a pseudonym, never the pad name,
// which can be what gives access to a pad.
func TestPadUsersHidePadNames(t *testing.T) {
	reset(t, true)
	const secretPad = "g.s8oes9dhwrvt0zif$unguessable-pad"
	SetPadUsers(map[string]int{secretPad: 1})
	reg := prometheus.NewRegistry()
	Register(reg)
	families, err := reg.Gather()
	require.NoError(t, err)
	var labels []string
	for _, f := range families {
		if f.GetName() != "etherpad_pad_users" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				labels = append(labels, l.GetName()+"="+l.GetValue())
			}
		}
	}
	require.Len(t, labels, 1)
	assert.NotContains(t, labels[0], "unguessable")
	assert.Regexp(t, `^pad=[0-9a-f]{12}$`, labels[0])
	assert.Equal(t, PadLabel(secretPad), PadLabel(secretPad), "stable within the process")
	assert.NotEqual(t, PadLabel("a"), PadLabel("b"))
}

func TestDisabledByDefault(t *testing.T) {
	reset(t, false)
	RecordSocketEmit("NEW_CHANGES")
	assert.Equal(t, 0, testutil.CollectAndCount(SocketEmitsTotal), "no-op")
	RecordChangesetApply()()
	assert.Equal(t, uint64(0), applyCount(t))
	SetPadUsers(map[string]int{"a": 1})
	assert.Equal(t, 0, testutil.CollectAndCount(PadUsers))

	reg := prometheus.NewRegistry()
	Register(reg)
	families, err := reg.Gather()
	require.NoError(t, err)
	assert.Empty(t, families, "nothing registered")

	SetEnabled(true)
	reg = prometheus.NewRegistry()
	Register(reg)
	RecordSocketEmit("NEW_CHANGES")
	families, err = reg.Gather()
	require.NoError(t, err)
	names := []string{}
	for _, f := range families {
		names = append(names, f.GetName())
	}
	assert.Contains(t, names, "etherpad_socket_emits_total")
}
