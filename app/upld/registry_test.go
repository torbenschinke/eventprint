package upld

import (
	"testing"
	"time"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/image"

	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/requirements/fun/upload"
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

// TestEnqueueKeepsInboxTemplate hält fest, dass ein leeres Layout leer bleibt.
//
// Vorher machte die Registry aus jedem unbekannten Layout "full" – auch aus
// dem leeren. Ein Bild für den Eingang wäre damit unverhofft gedruckt worden.
// Ein gesetztes, aber unbekanntes Layout wird dagegen weiter normalisiert.
func TestEnqueueKeepsInboxTemplate(t *testing.T) {
	tests := []struct {
		name string
		in   printing.TemplateID
		want printing.TemplateID
	}{
		{name: "inbox", in: InboxTemplate, want: InboxTemplate},
		{name: "known", in: printing.TemplatePassepartout, want: printing.TemplatePassepartout},
		{name: "unknown", in: "gibt-es-nicht", want: printing.TemplateFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry(nil)
			id, _ := r.Open("box")
			jobID, _ := NewJobID()
			if err := r.Enqueue(id, Job{ID: jobID, Image: "img", Template: tt.in}); err != nil {
				t.Fatal(err)
			}

			jobs, err := r.Pending("box")
			if err != nil {
				t.Fatal(err)
			}

			if len(jobs) != 1 || jobs[0].Template != tt.want {
				t.Fatalf("Layout %q wurde zu %q, erwartet %q", tt.in, jobs[0].Template, tt.want)
			}
		})
	}

	spec.Verified(t, upload.RUploadEingang)
}

func TestRemainingCountsDownToFull(t *testing.T) {
	r := NewRegistry(nil)
	id, _ := r.Open("box")

	if n, err := r.Remaining(id); err != nil || n != MaxJobsPerSession {
		t.Fatalf("Remaining = %d, %v; erwartet %d", n, err, MaxJobsPerSession)
	}

	for range MaxJobsPerSession {
		jobID, _ := NewJobID()
		if err := r.Enqueue(id, Job{ID: jobID, Image: image.ID(jobID)}); err != nil {
			t.Fatal(err)
		}
	}

	if n, _ := r.Remaining(id); n != 0 {
		t.Fatalf("Remaining = %d bei voller Warteschlange", n)
	}

	jobID, _ := NewJobID()
	if err := r.Enqueue(id, Job{ID: jobID, Image: "zuviel"}); err != ErrFull {
		t.Fatalf("Enqueue auf volle Warteschlange: %v, erwartet ErrFull", err)
	}

	if _, err := r.Remaining("gibt-es-nicht"); err != ErrExpired {
		t.Fatalf("Remaining einer unbekannten Sitzung: %v, erwartet ErrExpired", err)
	}

	spec.Verified(t, upload.RUploadEingang)
}
