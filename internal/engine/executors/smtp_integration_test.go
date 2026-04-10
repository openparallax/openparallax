package executors

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// smtpRecorder is a minimal in-process SMTP server. It speaks just enough of
// the protocol for the executor's Send path: greeting, EHLO, optional
// STARTTLS, AUTH PLAIN, MAIL FROM, RCPT TO, DATA, QUIT. Every command the
// client issues is recorded in the order received so tests can assert that
// authentication only happens after the channel is encrypted.
type smtpRecorder struct {
	listener net.Listener
	host     string
	port     int

	tlsConfig *tls.Config // nil for plaintext-only servers

	mu       sync.Mutex
	commands []string
	mailFrom string
	rcptTo   []string
	dataBody string
	authSeen bool
	tlsSeen  bool
}

func startSMTPRecorder(t *testing.T, withTLS bool) *smtpRecorder {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	rec := &smtpRecorder{
		listener: ln,
		host:     "127.0.0.1",
		port:     ln.Addr().(*net.TCPAddr).Port,
	}
	if withTLS {
		rec.tlsConfig = generateSelfSignedTLS(t, rec.host)
	}

	go rec.serve()
	t.Cleanup(func() { _ = ln.Close() })

	// Give the listener a moment to enter Accept.
	time.Sleep(20 * time.Millisecond)
	return rec
}

func (r *smtpRecorder) record(cmd string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = append(r.commands, cmd)
}

func (r *smtpRecorder) serve() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			return
		}
		go r.handle(conn)
	}
}

func (r *smtpRecorder) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	br := bufio.NewReader(conn)
	bw := bufio.NewWriter(conn)
	write := func(line string) {
		_, _ = bw.WriteString(line + "\r\n")
		_ = bw.Flush()
	}

	write("220 test.local ESMTP")

	inData := false
	var dataBuf strings.Builder

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				r.mu.Lock()
				r.dataBody = dataBuf.String()
				r.mu.Unlock()
				write("250 2.0.0 Ok")
				continue
			}
			dataBuf.WriteString(line)
			dataBuf.WriteString("\r\n")
			continue
		}

		r.record(line)
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			write("250-test.local")
			if r.tlsConfig != nil {
				r.mu.Lock()
				tlsAlready := r.tlsSeen
				r.mu.Unlock()
				if !tlsAlready {
					write("250-STARTTLS")
				}
			}
			write("250 AUTH PLAIN")
		case strings.HasPrefix(upper, "STARTTLS"):
			if r.tlsConfig == nil {
				write("502 STARTTLS not supported")
				continue
			}
			write("220 Ready to start TLS")
			tlsConn := tls.Server(conn, r.tlsConfig)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			r.mu.Lock()
			r.tlsSeen = true
			r.mu.Unlock()
			conn = tlsConn
			br = bufio.NewReader(conn)
			bw = bufio.NewWriter(conn)
			write = func(line string) {
				_, _ = bw.WriteString(line + "\r\n")
				_ = bw.Flush()
			}
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			r.mu.Lock()
			r.authSeen = true
			r.mu.Unlock()
			write("235 2.7.0 Authentication successful")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			r.mu.Lock()
			r.mailFrom = strings.TrimPrefix(line, "MAIL FROM:")
			r.mu.Unlock()
			write("250 2.1.0 Ok")
		case strings.HasPrefix(upper, "RCPT TO:"):
			r.mu.Lock()
			r.rcptTo = append(r.rcptTo, strings.TrimPrefix(line, "RCPT TO:"))
			r.mu.Unlock()
			write("250 2.1.5 Ok")
		case upper == "DATA":
			write("354 End data with <CR><LF>.<CR><LF>")
			inData = true
		case upper == "QUIT":
			write("221 2.0.0 Bye")
			return
		case upper == "RSET":
			write("250 2.0.0 Ok")
		case upper == "NOOP":
			write("250 2.0.0 Ok")
		default:
			write("502 Command not implemented")
		}
	}
}

func generateSelfSignedTLS(t *testing.T, host string) *tls.Config {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP(host)},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	cert := tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
}

// --- Tests ---

func TestSMTPIntegration_LoopbackPlaintextSend(t *testing.T) {
	rec := startSMTPRecorder(t, false)

	cfg := types.SMTPConfig{
		Host:     rec.host,
		Port:     rec.port,
		Username: "alice",
		Password: "supersecret",
		From:     "alice@example.com",
		TLS:      false,
	}
	provider := &smtpProvider{cfg: cfg}
	err := provider.Send(context.Background(), &Email{
		To:      []string{"bob@example.com"},
		Subject: "Hi",
		Body:    "Hello there",
	})
	require.NoError(t, err)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	assert.Contains(t, rec.mailFrom, "alice@example.com")
	require.Len(t, rec.rcptTo, 1)
	assert.Contains(t, rec.rcptTo[0], "bob@example.com")
	assert.Contains(t, rec.dataBody, "Subject: Hi")
	assert.Contains(t, rec.dataBody, "Hello there")
	// Loopback exemption: TLS not used, but auth still allowed.
	assert.False(t, rec.tlsSeen, "TLS should not have been negotiated")
}

func TestSMTPIntegration_NonLoopbackPlaintextRefused(t *testing.T) {
	cfg := types.SMTPConfig{
		Host:     "smtp.example.com", // not loopback
		Port:     25,
		Username: "alice",
		Password: "supersecret",
		From:     "alice@example.com",
		TLS:      false,
	}
	provider := &smtpProvider{cfg: cfg}
	err := provider.Send(context.Background(), &Email{
		To:      []string{"bob@example.com"},
		Subject: "x",
		Body:    "y",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TLS is disabled")
}

func TestSMTPIntegration_StartTLSAuthOrder(t *testing.T) {
	rec := startSMTPRecorder(t, true)

	cfg := types.SMTPConfig{
		Host:     rec.host,
		Port:     rec.port,
		Username: "alice",
		Password: "supersecret",
		From:     "alice@example.com",
		TLS:      true,
	}
	provider := &smtpProvider{
		cfg:               cfg,
		tlsConfigOverride: &tls.Config{ServerName: rec.host, InsecureSkipVerify: true}, //nolint:gosec // self-signed test cert
	}
	err := provider.Send(context.Background(), &Email{
		To:      []string{"bob@example.com"},
		Subject: "Hello",
		Body:    "Body text",
	})
	require.NoError(t, err)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	assert.True(t, rec.tlsSeen, "STARTTLS must have been negotiated")
	assert.True(t, rec.authSeen, "AUTH PLAIN must have run")

	// Auth must appear after STARTTLS in the command stream.
	tlsIdx := -1
	authIdx := -1
	for i, c := range rec.commands {
		if strings.HasPrefix(strings.ToUpper(c), "STARTTLS") {
			tlsIdx = i
		}
		if strings.HasPrefix(strings.ToUpper(c), "AUTH") {
			authIdx = i
		}
	}
	require.NotEqual(t, -1, tlsIdx)
	require.NotEqual(t, -1, authIdx)
	assert.Less(t, tlsIdx, authIdx, "AUTH must come after STARTTLS")
}

func TestSMTPIntegration_EmptyCredentialFailsClosed(t *testing.T) {
	cfg := types.SMTPConfig{
		Host:     "127.0.0.1",
		Port:     25,
		Username: "$NEVER_SET_OPENPARALLAX_TEST_VAR",
		Password: "literal",
		From:     "alice@example.com",
		TLS:      false,
	}
	provider := &smtpProvider{cfg: cfg}
	err := provider.Send(context.Background(), &Email{
		To:      []string{"bob@example.com"},
		Subject: "x",
		Body:    "y",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username is empty")
}

func TestSMTPIntegration_InvalidFromRejected(t *testing.T) {
	cfg := types.SMTPConfig{
		Host:     "127.0.0.1",
		Port:     25,
		Username: "alice",
		Password: "supersecret",
		From:     "not a valid address",
		TLS:      false,
	}
	provider := &smtpProvider{cfg: cfg}
	err := provider.Send(context.Background(), &Email{
		To:      []string{"bob@example.com"},
		Subject: "x",
		Body:    "y",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestSMTPIntegration_RecipientValidationAtExecutorLayer(t *testing.T) {
	exec := newEmailExecutorWithMockReader(&mockIMAPReader{})
	exec.provider = &mockMailProvider{}
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSendEmail,
		Payload: map[string]any{
			"to":      "garbage, also-garbage",
			"subject": "x",
			"body":    "y",
		},
	})
	require.False(t, result.Success)
	assert.Contains(t, result.Error, "invalid recipient")
}

// Compile-time guard against accidentally importing fmt without a use site.
var _ = fmt.Sprintf
