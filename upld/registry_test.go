package upld

import (
	"testing"
	"time"

	"go.wdy.de/nago/application/image"
)

func TestOpenRotatesIdentityAndPurgesImages(t *testing.T) {
	purged := make(chan image.ID, 1)
	r := NewRegistry(func(id image.ID) { purged <- id })
	first, err := r.Open("box")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Track(first, "img"); err != nil {
		t.Fatal(err)
	}
	second, err := r.Open("box")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || r.Valid(first) || !r.Valid(second) {
		t.Fatal("opening a session did not rotate its identity")
	}
	select {
	case id := <-purged:
		if id != "img" {
			t.Fatalf("purged %q, want img", id)
		}
	case <-time.After(time.Second):
		t.Fatal("old image was not purged")
	}
}

func TestTokenCannotReadOtherQueue(t *testing.T) {
	r := NewRegistry(nil)
	id, _ := r.Open("box-a")
	jobID, _ := NewJobID()
	if err := r.Enqueue(id, Job{ID: jobID, Image: "img"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Pending("box-b"); err == nil {
		t.Fatal("foreign token can read queue")
	}
	if _, ok := r.Find("box-b", jobID); ok {
		t.Fatal("foreign token can read image")
	}
}

func TestPurgeExpiresSession(t *testing.T) {
	r := NewRegistry(nil)
	id, _ := r.Open("box")
	r.PurgeOlderThan(time.Now().Add(time.Second))
	if r.Valid(id) {
		t.Fatal("expired session remains valid")
	}
}

func TestTrackPurgesImageWhenSessionExpired(t *testing.T) {
	purged := make(chan image.ID, 1)
	r := NewRegistry(func(id image.ID) { purged <- id })
	if err := r.Track("missing", "orphan"); err != ErrExpired {
		t.Fatalf("Track returned %v, want ErrExpired", err)
	}
	select {
	case id := <-purged:
		if id != "orphan" {
			t.Fatalf("purged %q, want orphan", id)
		}
	case <-time.After(time.Second):
		t.Fatal("orphan was not purged")
	}
}

// TestPurgeKeepsUsedSession hält fest, dass die Verfallsfrist am letzten
// Zugriff hängt und nicht am Alter der Sitzung.
//
// Vorher wurde eine Sitzung nach 30 Minuten verworfen, gleich ob sie benutzt
// wurde. Auf einer Feier wechselte der QR-Code dadurch mitten im Betrieb, und
// wer ihn kurz zuvor gescannt hatte, lud ins Leere.
func TestPurgeKeepsUsedSession(t *testing.T) {
	r := NewRegistry(nil)
	id, _ := r.Open("box")

	// Die Fotobox fragt ihre Warteschlange ab – das ist ein Zugriff.
	if _, err := r.Pending("box"); err != nil {
		t.Fatalf("Pending: %v", err)
	}

	// Der Stichtag liegt vor diesem Zugriff, aber nach der Eröffnung.
	r.PurgeOlderThan(time.Now().Add(-time.Millisecond))

	if !r.Valid(id) {
		t.Fatal("eine benutzte Sitzung wurde verworfen")
	}
}
