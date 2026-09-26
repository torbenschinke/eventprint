package nas

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// SMB ist der Client für echte Freigaben.
//
// Er hält eine Sitzung offen, solange sich die Zugangsdaten nicht ändern. Eine
// Anmeldung kostet mehrere Rundreisen, und die Galerie fragt Dutzende
// Vorschaubilder gleichzeitig an; mit einer Anmeldung je Bild wäre das NAS
// eine Diashow. Bricht die Verbindung ab, weil das NAS schlafen ging oder neu
// startete, baut der nächste Zugriff sie neu auf.
type SMB struct {
	mu   sync.Mutex
	cfg  Config
	conn net.Conn
	sess *smb2.Session
	fs   *smb2.Share
}

// NewSMB liefert einen Client ohne offene Verbindung.
func NewSMB() *SMB { return &SMB{} }

// dialTimeout begrenzt den Verbindungsaufbau. Ein NAS im Heimnetz antwortet
// in Millisekunden; wer länger wartet, wartet auf ein ausgeschaltetes Gerät.
const dialTimeout = 5 * time.Second

// Shares meldet sich mit einer eigenen, kurzen Sitzung an. Sie dient dem
// Ausprobieren von Zugangsdaten und soll die offene Sitzung nicht ersetzen,
// solange niemand gespeichert hat.
func (c *SMB) Shares(ctx context.Context, cfg Config) ([]string, error) {
	conn, sess, err := dial(ctx, cfg)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = sess.Logoff()
		_ = conn.Close()
	}()

	names, err := sess.WithContext(ctx).ListSharenames()
	if err != nil {
		return nil, translate(err)
	}

	return names, nil
}

// Do führt fn auf der Freigabe aus. Scheitert fn an einer abgerissenen
// Verbindung, wird einmal neu verbunden und fn wiederholt; die
// Anwendungsfälle lesen nur und vertragen das.
func (c *SMB) Do(ctx context.Context, cfg Config, fn func(fsys fs.FS) error) error {
	for attempt := 0; ; attempt++ {
		share, err := c.mount(ctx, cfg)
		if err != nil {
			return err
		}

		err = fn(share.WithContext(ctx).DirFS(""))
		if err == nil {
			return nil
		}

		if attempt > 0 || ctx.Err() != nil || !broken(err) {
			return translate(err)
		}

		slog.Info("nas connection lost, reconnecting", "host", cfg.Host, "err", err)
		c.drop(share)
	}
}

// mount liefert die offene Freigabe oder verbindet neu.
func (c *SMB) mount(ctx context.Context, cfg Config) (*smb2.Share, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.fs != nil && c.cfg == cfg {
		return c.fs, nil
	}

	c.closeLocked()

	conn, sess, err := dial(ctx, cfg)
	if err != nil {
		return nil, err
	}

	share, err := sess.WithContext(ctx).Mount(cfg.Share)
	if err != nil {
		_ = sess.Logoff()
		_ = conn.Close()
		return nil, translate(err)
	}

	c.cfg, c.conn, c.sess, c.fs = cfg, conn, sess, share

	return share, nil
}

// drop verwirft die Sitzung, wenn sie noch die gescheiterte ist. Eine
// parallele Anfrage hat sie womöglich schon ersetzt.
func (c *SMB) drop(share *smb2.Share) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.fs == share {
		c.closeLocked()
	}
}

// Close trennt die Verbindung.
func (c *SMB) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closeLocked()

	return nil
}

func (c *SMB) closeLocked() {
	if c.fs != nil {
		_ = c.fs.Umount()
	}

	if c.sess != nil {
		_ = c.sess.Logoff()
	}

	if c.conn != nil {
		_ = c.conn.Close()
	}

	c.cfg, c.conn, c.sess, c.fs = Config{}, nil, nil, nil
}

// dial baut Verbindung und Sitzung auf.
func dial(ctx context.Context, cfg Config) (net.Conn, *smb2.Session, error) {
	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "tcp", cfg.address())
	if err != nil {
		return nil, nil, failure{ErrUnreachable, err}
	}

	domain, user := cfg.credentials()
	sd := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: user, Password: cfg.Password, Domain: domain}}

	sess, err := sd.DialContext(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, nil, translate(err)
	}

	return conn, sess, nil
}

// NTSTATUS-Werte, die eine Bedeutung für die Oberfläche haben. go-smb2 führt
// sie nur in einem internen Paket.
const (
	statusAccessDenied        = 0xC0000022
	statusLogonFailure        = 0xC000006D
	statusAccountRestriction  = 0xC000006E
	statusPasswordExpired     = 0xC0000071
	statusAccountDisabled     = 0xC0000072
	statusNetworkNameDeleted  = 0xC00000C9
	statusNetworkAccessDenied = 0xC00000CA
	statusBadNetworkName      = 0xC00000CC
	statusUserSessionDeleted  = 0xC0000203
	statusSessionExpired      = 0xC000035C
	statusAccountLockedOut    = 0xC0000234
)

// translate macht aus einem Fehler des Protokolls eine Meldung für Menschen.
// Was keine eigene Bedeutung hat – etwa fs.ErrNotExist –, bleibt, wie es ist.
func translate(err error) error {
	if err == nil {
		return nil
	}

	var f failure
	if errors.As(err, &f) {
		return err
	}

	var re *smb2.ResponseError
	if errors.As(err, &re) {
		switch re.Code {
		case statusLogonFailure, statusAccountRestriction, statusPasswordExpired, statusAccountDisabled, statusAccountLockedOut:
			return failure{ErrLogin, err}
		case statusBadNetworkName:
			return failure{ErrShare, err}
		case statusAccessDenied, statusNetworkAccessDenied:
			return failure{ErrDenied, err}
		}
	}

	if broken(err) {
		return failure{ErrUnreachable, err}
	}

	return err
}

// broken erkennt eine Verbindung, die nicht mehr zu gebrauchen ist.
func broken(err error) bool {
	var te *smb2.TransportError
	var ie *smb2.InvalidResponseError
	var ne net.Error
	if errors.As(err, &te) || errors.As(err, &ie) || errors.As(err, &ne) {
		return true
	}

	var re *smb2.ResponseError
	if errors.As(err, &re) {
		switch re.Code {
		case statusNetworkNameDeleted, statusUserSessionDeleted, statusSessionExpired:
			return true
		}
	}

	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}
