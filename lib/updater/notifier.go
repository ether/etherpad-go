package updater

import (
	"fmt"
	"time"
)

// EmailLog is what the notifier has sent, so a notice is not repeated on
// every check (etherpad-lite's email state).
type EmailLog struct {
	SevereAt       string `json:"severeAt,omitempty"`       // RFC3339, last "outdated" mail
	GraceStartTag  string `json:"graceStartTag,omitempty"`  // release a "scheduled" mail went out for
	LastFailureKey string `json:"lastFailureKey,omitempty"` // "<outcome>:<tag>" of the last failure mail
}

// Email is one planned mail.
type Email struct {
	Kind    string
	Subject string
	Body    string
}

// severeInterval is how often the "outdated" mail repeats.
const severeInterval = 30 * 24 * time.Hour

// adminPath is where the admin page shows the updater.
const adminPath = "/admin/updates"

// DecideEmails plans the "outdated" mail: when the instance is a minor
// version or more behind, at most every 30 days. It is pure: the caller
// sends and stores the new log.
func DecideEmails(adminEmail, current, latest string, now time.Time, log EmailLog) ([]Email, EmailLog) {
	if adminEmail == "" || !isMinorOrMoreBehind(current, latest) {
		return nil, log
	}
	if last, err := time.Parse(time.RFC3339, log.SevereAt); err == nil && now.Sub(last) < severeInterval {
		return nil, log
	}
	log.SevereAt = now.UTC().Format(time.RFC3339)
	return []Email{{
		Kind:    "severe",
		Subject: fmt.Sprintf("[Etherpad] Your instance is outdated (%s)", current),
		Body: fmt.Sprintf("Your Etherpad version (%s) is at least one minor release behind the latest published version (%s). Consider scheduling an upgrade.",
			current, latest),
	}}, log
}

// DecideGraceStartEmail plans the "update scheduled" mail, once per release.
func DecideGraceStartEmail(adminEmail, current string, target ReleaseInfo, at time.Time, log EmailLog) ([]Email, EmailLog) {
	if adminEmail == "" || log.GraceStartTag == target.Tag {
		return nil, log
	}
	log.GraceStartTag = target.Tag
	return []Email{{
		Kind:    "grace-start",
		Subject: fmt.Sprintf("[Etherpad] Auto-update scheduled for %s", target.Version),
		Body: fmt.Sprintf("Etherpad will auto-update to %s at %s. Your version is %s. To cancel, visit %s.",
			target.Tag, at.UTC().Format(time.RFC3339), current, adminPath),
	}}, log
}

// DecideOutcomeEmail plans the mail about a failed apply. The same outcome
// for the same release is mailed once (an apply failing the same way again
// does not mail again); a failed rollback always is: someone has to act.
func DecideOutcomeEmail(adminEmail, outcome, reason, targetTag, currentVersion string, log EmailLog) ([]Email, EmailLog) {
	if adminEmail == "" {
		return nil, log
	}
	key := outcome + ":" + targetTag
	terminal := outcome == "rollback-failed"
	if !terminal && log.LastFailureKey == key {
		return nil, log
	}
	var kind, subject string
	switch outcome {
	case "preflight-failed":
		kind, subject = "update-preflight-failed", fmt.Sprintf("[Etherpad] Auto-update to %s blocked at preflight", targetTag)
	case "rolled-back":
		kind, subject = "update-rolled-back", fmt.Sprintf("[Etherpad] Auto-update to %s rolled back", targetTag)
	case "rollback-failed":
		kind, subject = "update-rollback-failed", "[Etherpad] Auto-update FAILED and could not be rolled back — manual intervention required"
	default:
		kind, subject = "update-failed", fmt.Sprintf("[Etherpad] Auto-update to %s failed", targetTag)
	}
	tail := fmt.Sprintf(" Visit %s for details.", adminPath)
	if terminal {
		tail = fmt.Sprintf(" Restore a working install, then acknowledge the failure on %s.", adminPath)
	}
	log.LastFailureKey = key
	return []Email{{
		Kind:    kind,
		Subject: subject,
		Body:    fmt.Sprintf("Etherpad attempted to update to %s but failed: %s.\nThe running version is %s.%s", targetTag, reason, currentVersion, tail),
	}}, log
}
