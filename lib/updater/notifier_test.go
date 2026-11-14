package updater

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's Notifier spec.

var mailNow = utc("2026-04-25T12:00:00Z")

func TestDecideEmails(t *testing.T) {
	mails, log := DecideEmails("", "2.0.0", "2.7.2", mailNow, EmailLog{})
	assert.Empty(t, mails, "no admin address")
	assert.Equal(t, EmailLog{}, log)

	mails, log = DecideEmails("admin@example.com", "2.0.0", "2.7.2", mailNow, EmailLog{})
	require.Len(t, mails, 1)
	assert.Equal(t, "severe", mails[0].Kind)
	assert.Contains(t, mails[0].Subject, "2.0.0")
	assert.Contains(t, mails[0].Body, "2.7.2")
	assert.Equal(t, "2026-04-25T12:00:00Z", log.SevereAt)

	mails, _ = DecideEmails("admin@example.com", "2.0.0", "2.7.2", mailNow, EmailLog{SevereAt: "2026-04-10T12:00:00Z"})
	assert.Empty(t, mails, "not again within 30 days")
	mails, _ = DecideEmails("admin@example.com", "2.0.0", "2.7.2", mailNow, EmailLog{SevereAt: "2026-03-20T12:00:00Z"})
	assert.Len(t, mails, 1, "again after 30 days")

	mails, _ = DecideEmails("admin@example.com", "2.7.1", "2.7.2", mailNow, EmailLog{})
	assert.Empty(t, mails, "a patch behind is not outdated")
	mails, _ = DecideEmails("admin@example.com", "dev", "2.7.2", mailNow, EmailLog{})
	assert.Empty(t, mails, "a dev build")
}

func TestDecideOutcomeEmail(t *testing.T) {
	const admin = "ops@example.com"
	mails, log := DecideOutcomeEmail("", "rolled-back", "x", "v2.7.6", "2.7.5", EmailLog{})
	assert.Empty(t, mails)
	assert.Equal(t, EmailLog{}, log)

	mails, log = DecideOutcomeEmail(admin, "rolled-back", "health-check-timeout", "v2.7.6", "2.7.5", EmailLog{})
	require.Len(t, mails, 1)
	assert.Equal(t, "update-rolled-back", mails[0].Kind)
	assert.Contains(t, mails[0].Subject, "v2.7.6")
	assert.Contains(t, mails[0].Body, "health-check-timeout")
	assert.Contains(t, mails[0].Body, "2.7.5")
	assert.Equal(t, "rolled-back:v2.7.6", log.LastFailureKey)

	mails, log = DecideOutcomeEmail(admin, "preflight-failed", "insufficient-disk-space", "v2.7.6", "2.7.5", EmailLog{})
	assert.Equal(t, "update-preflight-failed", mails[0].Kind)
	assert.Contains(t, mails[0].Body, "insufficient-disk-space")
	assert.Equal(t, "preflight-failed:v2.7.6", log.LastFailureKey)

	mails, _ = DecideOutcomeEmail(admin, "rollback-failed", "rename failed", "v2.7.6", "2.7.5", EmailLog{})
	assert.Equal(t, "update-rollback-failed", mails[0].Kind)
	assert.Contains(t, mails[0].Subject, "manual intervention")
	assert.Contains(t, mails[0].Body, "acknowledge")

	mails, _ = DecideOutcomeEmail(admin, "failed", "checksum mismatch", "v2.7.6", "2.7.5", EmailLog{})
	assert.Equal(t, "update-failed", mails[0].Kind)

	_, first := DecideOutcomeEmail(admin, "rolled-back", "x", "v2.7.6", "2.7.5", EmailLog{})
	mails, second := DecideOutcomeEmail(admin, "rolled-back", "x", "v2.7.6", "2.7.5", first)
	assert.Empty(t, mails, "the same failure again")
	assert.Equal(t, first, second)

	_, first = DecideOutcomeEmail(admin, "preflight-failed", "x", "v2.7.6", "2.7.5", EmailLog{})
	mails, second = DecideOutcomeEmail(admin, "rolled-back", "x", "v2.7.6", "2.7.5", first)
	assert.Len(t, mails, 1, "another outcome, same release")
	assert.Equal(t, "rolled-back:v2.7.6", second.LastFailureKey)

	_, first = DecideOutcomeEmail(admin, "rolled-back", "x", "v2.7.6", "2.7.5", EmailLog{})
	mails, second = DecideOutcomeEmail(admin, "rolled-back", "x", "v2.7.7", "2.7.5", first)
	assert.Len(t, mails, 1, "the same outcome, another release")
	assert.Equal(t, "rolled-back:v2.7.7", second.LastFailureKey)

	_, first = DecideOutcomeEmail(admin, "rollback-failed", "x", "v2.7.6", "2.7.5", EmailLog{})
	mails, _ = DecideOutcomeEmail(admin, "rollback-failed", "x", "v2.7.6", "2.7.5", first)
	assert.Len(t, mails, 1, "a failed rollback always mails")
}

func TestDecideGraceStartEmail(t *testing.T) {
	target := ReleaseInfo{Version: "2.0.1", Tag: "v2.0.1"}
	at := utc("2026-05-11T12:15:00Z")
	mails, log := DecideGraceStartEmail("admin@example.com", "2.0.0", target, at, EmailLog{})
	require.Len(t, mails, 1)
	assert.Equal(t, "grace-start", mails[0].Kind)
	assert.Contains(t, mails[0].Body, "2026-05-11T12:15:00Z")
	assert.Contains(t, mails[0].Body, adminPath)
	assert.Equal(t, "v2.0.1", log.GraceStartTag)

	mails, _ = DecideGraceStartEmail("admin@example.com", "2.0.0", target, at, log)
	assert.Empty(t, mails, "once per release (a restart during the grace period)")
	mails, _ = DecideGraceStartEmail("admin@example.com", "2.0.0", ReleaseInfo{Version: "2.0.2", Tag: "v2.0.2"}, at, log)
	assert.Len(t, mails, 1, "a newer release")
	mails, _ = DecideGraceStartEmail("", "2.0.0", target, at, EmailLog{})
	assert.Empty(t, mails)
}

type sentMail struct{ to, subject string }

func mailingUpdater(t *testing.T) (*Updater, func() []sentMail) {
	u, _ := newTestUpdater(t, filepath.Join(t.TempDir(), "etherpad"))
	var mu sync.Mutex
	var sent []sentMail
	u.cfg.AdminEmail = func() string { return " admin@example.com " }
	u.cfg.SendMail = func(to, subject, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, sentMail{to, subject})
		return nil
	}
	return u, func() []sentMail {
		// Background sends: wait until they are in.
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return append([]sentMail(nil), sent...)
	}
}

// The updater mails at the right moments, once each.
func TestUpdaterMails(t *testing.T) {
	u, sent := mailingUpdater(t)
	u.cfg.Tier = TierAuto
	u.cfg.GraceMinutes = 15
	u.state = EmptyState()
	u.state.Latest = &ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}

	u.mu.Lock()
	u.decideScheduleLocked()
	u.decideScheduleLocked()
	u.mu.Unlock()
	u.Stop()
	got := sent()
	require.Len(t, got, 1, "one scheduled mail")
	assert.Equal(t, "admin@example.com", got[0].to)
	assert.Contains(t, got[0].subject, "scheduled")

	u.recordResult(ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}, "preflight-failed", "insufficient-disk-space")
	u.recordResult(ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}, "preflight-failed", "insufficient-disk-space")
	got = sent()
	require.Len(t, got, 2, "one failure mail")
	assert.True(t, strings.Contains(got[1].subject, "preflight"))
	assert.Equal(t, "preflight-failed:v2.0.0", LoadState(u.cfg.StatePath).Email.LastFailureKey, "the log is saved")

	u.recordResult(ReleaseInfo{Version: "2.0.0", Tag: "v2.0.0"}, "verified", "")
	assert.Len(t, sent(), 2, "no mail for a verified update")
}

func TestUpdaterTestMail(t *testing.T) {
	u, sent := mailingUpdater(t)
	require.NoError(t, u.TestMail())
	assert.Equal(t, []sentMail{{"admin@example.com", "[Etherpad] Test mail"}}, sent())
	u.cfg.AdminEmail = func() string { return "" }
	assert.ErrorIs(t, u.TestMail(), ErrNoAdminEmail)
	assert.Equal(t, "", u.Status().AdminEmail)
}
