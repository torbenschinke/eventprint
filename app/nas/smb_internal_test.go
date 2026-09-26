package nas

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"testing"

	"github.com/hirochachacha/go-smb2"
)

// Die Meldungen des Protokolls werden zu Sätzen, mit denen jemand vor der Box
// etwas anfangen kann; die Ursache bleibt für das Protokoll erhalten.
func TestTranslate(t *testing.T) {
	cases := []struct {
		err  error
		want error
	}{
		{&smb2.ResponseError{Code: statusLogonFailure}, ErrLogin},
		{&smb2.ResponseError{Code: statusAccountDisabled}, ErrLogin},
		{fmt.Errorf("mount: %w", &smb2.ResponseError{Code: statusBadNetworkName}), ErrShare},
		{&fs.PathError{Op: "open", Path: "x", Err: &smb2.ResponseError{Code: statusAccessDenied}}, ErrDenied},
		{&smb2.TransportError{Err: io.EOF}, ErrUnreachable},
		{&net.OpError{Op: "dial", Err: errors.New("no route to host")}, ErrUnreachable},
	}

	for _, c := range cases {
		got := translate(c.err)
		if !errors.Is(got, c.want) || !errors.Is(got, c.err) || got.Error() != c.want.Error() {
			t.Errorf("translate(%v) = %v, want %v wrapping the cause", c.err, got, c.want)
		}
	}

	if got := translate(fs.ErrNotExist); got != fs.ErrNotExist {
		t.Errorf("not-exist must stay as is, got %v", got)
	}

	if !broken(&smb2.ResponseError{Code: statusNetworkNameDeleted}) || broken(&smb2.ResponseError{Code: statusAccessDenied}) {
		t.Error("only a dead session is broken, not a missing right")
	}
}

func TestConfigAddress(t *testing.T) {
	for in, want := range map[string]string{
		"diskstation.local": "diskstation.local:445",
		"192.168.1.20:4445": "192.168.1.20:4445",
		"fe80::1":           "[fe80::1]:445",
		"[fe80::1]":         "[fe80::1]:445",
	} {
		if got := (Config{Host: in}).address(); got != want {
			t.Errorf("address(%q) = %q, want %q", in, got, want)
		}
	}

	if d, u := (Config{User: `HEIM\anna`}).credentials(); d != "HEIM" || u != "anna" {
		t.Errorf("credentials = %q, %q", d, u)
	}
}
