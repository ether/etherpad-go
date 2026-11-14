// Package mail sends plain-text notices through an SMTP server (the
// updater's mails to the admin). Without a host it only logs what it would
// send, as etherpad-lite does.
package mail

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"go.uber.org/zap"
)

const defaultPort = 587

// Config is the SMTP part of the settings.
type Config = settings.MailSettings

// Port is the configured port, 587 when unset.
func Port(c Config) int {
	if c.Port <= 0 || c.Port > 65535 {
		return defaultPort
	}
	return c.Port
}

// TransportKey identifies the connection settings: when it changes (host,
// port, TLS or login, after settings were saved) a new client is built.
// etherpad-lite's smtpTransportKey; a host-only key would keep rotated
// credentials or a new port from taking effect.
func TransportKey(c Config) string {
	var auth any
	if c.Auth != nil {
		auth = *c.Auth
	}
	b, _ := json.Marshal(map[string]any{"host": c.Host, "port": Port(c), "secure": c.Secure, "auth": auth})
	return string(b)
}

// Mailer sends mail with the settings it reads at every send, so saved
// settings apply without a restart.
type Mailer struct {
	Settings func() Config
	Logger   *zap.SugaredLogger
	// dial is the network connection; tests replace it.
	dial func(network, addr string, tlsConfig *tls.Config) (net.Conn, error)
}

// New builds a mailer.
func New(cfg func() Config, logger *zap.SugaredLogger) *Mailer {
	return &Mailer{Settings: cfg, Logger: logger}
}

// headerSafe refuses CR and LF: a subject or address with them would add
// headers (header injection).
func headerSafe(values ...string) error {
	for _, v := range values {
		if strings.ContainsAny(v, "\r\n") {
			return errors.New("mail: line break in a header value")
		}
	}
	return nil
}

// Message renders a plain-text message.
func Message(from, to, subject, body string, now time.Time) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\n", from, to, subject, now.Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	body = strings.ReplaceAll(body, "\r\n", "\n")
	// Dot-stuffing is the job of net/smtp's DATA writer.
	for _, line := range strings.Split(body, "\n") {
		b.WriteString(line + "\r\n")
	}
	return []byte(b.String())
}

// Send delivers one mail. Without a host or sender it logs the mail and
// returns nil: the notice was "sent" as far as the caller's dedupe goes.
func (m *Mailer) Send(to, subject, body string) error {
	c := m.Settings()
	if c.Host == "" || c.From == "" {
		if m.Logger != nil {
			m.Logger.Infof("mail: (would send) to=%s subject=%q", to, subject)
		}
		return nil
	}
	if err := headerSafe(to, subject, c.From); err != nil {
		return err
	}
	err := m.send(c, to, Message(c.From, to, subject, body, time.Now()))
	if m.Logger != nil {
		if err != nil {
			m.Logger.Warnf("mail: sending to %s failed: %v", to, err)
		} else {
			m.Logger.Infof("mail: sent to=%s subject=%q", to, subject)
		}
	}
	return err
}

func (m *Mailer) send(c Config, to string, msg []byte) error {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(Port(c)))
	tlsConfig := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	dial := m.dial
	if dial == nil {
		dial = func(network, addr string, tc *tls.Config) (net.Conn, error) {
			d := &net.Dialer{Timeout: 30 * time.Second}
			if tc != nil {
				return tls.DialWithDialer(d, network, addr, tc)
			}
			return d.Dial(network, addr)
		}
	}
	var conn net.Conn
	var err error
	if c.Secure {
		conn, err = dial("tcp", addr, tlsConfig)
	} else {
		conn, err = dial("tcp", addr, nil)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(time.Minute))
	client, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()
	if !c.Secure {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return err
			}
		}
	}
	if c.Auth != nil && c.Auth.User != "" {
		// PlainAuth refuses to send the password over an unencrypted
		// connection to a host other than localhost.
		if err := client.Auth(smtp.PlainAuth("", c.Auth.User, c.Auth.Pass, c.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(c.From); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
