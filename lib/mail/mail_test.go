package mail

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's smtpTransportKey spec.
func TestTransportKey(t *testing.T) {
	base := Config{Host: "smtp.example.com", Port: 587}
	assert.NotEqual(t, TransportKey(base), TransportKey(Config{Host: "smtp.example.com", Port: 465}), "port")
	assert.NotEqual(t, TransportKey(base), TransportKey(Config{Host: "smtp.example.com", Port: 587, Secure: true}), "secure")
	withAuth := func(pass string) Config {
		return Config{Host: "smtp.example.com", Port: 587, Auth: &settings.MailAuth{User: "a", Pass: pass}}
	}
	assert.NotEqual(t, TransportKey(withAuth("1")), TransportKey(withAuth("2")), "credentials")
	assert.Equal(t, TransportKey(withAuth("1")), TransportKey(withAuth("1")), "stable")
	assert.Equal(t, TransportKey(Config{Host: "h"}), TransportKey(Config{Host: "h", Port: 587}), "587 by default")
	assert.Equal(t, TransportKey(Config{Host: "h", Port: -3}), TransportKey(Config{Host: "h", Port: 587}))
	assert.Equal(t, TransportKey(Config{Host: "h", Port: 70000}), TransportKey(Config{Host: "h", Port: 587}))
	assert.NotEqual(t, TransportKey(Config{Host: "a"}), TransportKey(Config{Host: "b"}), "host")
}

func TestSendWithoutHostOnlyLogs(t *testing.T) {
	dialed := false
	m := New(func() Config { return Config{From: "e@x"} }, nil)
	m.dial = func(string, string, *tls.Config) (net.Conn, error) { dialed = true; return nil, nil }
	require.NoError(t, m.Send("admin@example.com", "s", "b"))
	m.Settings = func() Config { return Config{Host: "smtp"} }
	require.NoError(t, m.Send("admin@example.com", "s", "b"), "no sender: logged only")
	assert.False(t, dialed)
}

func TestHeaderInjection(t *testing.T) {
	m := New(func() Config { return Config{Host: "smtp", From: "e@x"} }, nil)
	m.dial = func(string, string, *tls.Config) (net.Conn, error) { t.Fatal("dialed"); return nil, nil }
	assert.Error(t, m.Send("admin@example.com", "hi\r\nBcc: victim@example.com", "b"))
	assert.Error(t, m.Send("a@x\nBcc: v@x", "s", "b"))
}

// fakeSMTP is a minimal SMTP server on a pipe that records the session.
type fakeSMTP struct {
	mu       sync.Mutex
	commands []string
	data     string
}

func (f *fakeSMTP) serve(conn net.Conn, auth bool) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	_ = tp.PrintfLine("220 fake ESMTP")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.commands = append(f.commands, line)
		f.mu.Unlock()
		switch cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0]); cmd {
		case "EHLO", "HELO":
			if auth {
				_ = tp.PrintfLine("250-fake\r\n250 AUTH PLAIN")
			} else {
				_ = tp.PrintfLine("250 fake")
			}
		case "AUTH":
			_ = tp.PrintfLine("235 ok")
		case "MAIL", "RCPT", "RSET", "NOOP":
			_ = tp.PrintfLine("250 ok")
		case "DATA":
			_ = tp.PrintfLine("354 go")
			b, _ := io.ReadAll(bufio.NewReader(tp.DotReader()))
			f.mu.Lock()
			f.data = string(b)
			f.mu.Unlock()
			_ = tp.PrintfLine("250 queued")
		case "QUIT":
			_ = tp.PrintfLine("221 bye")
			return
		default:
			_ = tp.PrintfLine("502 no")
		}
	}
}

func TestSendDelivers(t *testing.T) {
	f := &fakeSMTP{}
	m := New(func() Config { return Config{Host: "localhost", Port: 2525, From: "etherpad@example.com"} }, nil)
	var addr string
	m.dial = func(_ string, a string, tc *tls.Config) (net.Conn, error) {
		addr = a
		assert.Nil(t, tc, "no TLS from the start without secure")
		client, server := net.Pipe()
		go f.serve(server, false)
		return client, nil
	}
	require.NoError(t, m.Send("admin@example.com", "[Etherpad] Test", "line one\n.leading dot\nend"))
	assert.Equal(t, "localhost:2525", addr)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Contains(t, f.commands, "MAIL FROM:<etherpad@example.com>")
	assert.Contains(t, f.commands, "RCPT TO:<admin@example.com>")
	// The DotReader hands lines back with \n and dot-stuffing undone.
	assert.Contains(t, f.data, "Subject: [Etherpad] Test\n")
	assert.Contains(t, f.data, "To: admin@example.com\n")
	assert.Contains(t, f.data, "\n\nline one\n.leading dot\nend\n", "the body, a leading dot intact")
}

func TestSendSecureDialsTLS(t *testing.T) {
	m := New(func() Config { return Config{Host: "smtp.example.com", Port: 465, Secure: true, From: "e@x"} }, nil)
	var got *tls.Config
	m.dial = func(_ string, _ string, tc *tls.Config) (net.Conn, error) { got = tc; return nil, assert.AnError }
	assert.Error(t, m.Send("a@x", "s", "b"))
	require.NotNil(t, got)
	assert.Equal(t, "smtp.example.com", got.ServerName)
	assert.GreaterOrEqual(t, got.MinVersion, uint16(tls.VersionTLS12))
}

// PlainAuth does not send a password over plain text to another host.
func TestNoPasswordInPlainText(t *testing.T) {
	f := &fakeSMTP{}
	m := New(func() Config {
		return Config{Host: "smtp.example.com", Port: 25, From: "e@x", Auth: &settings.MailAuth{User: "u", Pass: "secret"}}
	}, nil)
	m.dial = func(string, string, *tls.Config) (net.Conn, error) {
		c, s := net.Pipe()
		go f.serve(s, true)
		return c, nil
	}
	assert.Error(t, m.Send("a@x", "s", "b"))
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.commands {
		assert.NotContains(t, c, "AUTH", "no credentials without TLS")
	}
}

func TestMessage(t *testing.T) {
	msg := string(Message("f@x", "t@x", "Sub", "a\r\nb", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)))
	assert.True(t, strings.HasPrefix(msg, "From: f@x\r\nTo: t@x\r\nSubject: Sub\r\nDate: Fri, 02 Jan 2026 03:04:05 +0000\r\n"))
	assert.Contains(t, msg, "Content-Type: text/plain; charset=UTF-8")
	assert.True(t, strings.HasSuffix(msg, "\r\n\r\na\r\nb\r\n"))
}
