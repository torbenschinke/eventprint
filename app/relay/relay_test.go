package relay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/relay"
)

var su = permission.SU()

// nobody hat keine einzige Berechtigung.
type nobody struct{}

func (nobody) Audit(id permission.ID) error     { return fmt.Errorf("permission denied: %s", id) }
func (nobody) HasPermission(permission.ID) bool { return false }

const token = "geheim"

// fakeRelay spielt den Upload-Dienst nach, so weit das Gerät ihn sieht: eine
// Sitzung, eine Liste wartender Aufträge, das Bild je Auftrag und die
// Bestätigung. Ein Auftrag bleibt liegen, bis er bestätigt ist – genau das
// Verhalten, auf das sich der Poller verlässt.
type fakeRelay struct {
	srv *httptest.Server

	mu       sync.Mutex
	pending  []relay.Job
	images   map[string][]byte
	ackFails map[string]int // so oft scheitert die Bestätigung noch
	acks     map[string]int // Bestätigungsversuche je Auftrag
	sessions int
	expire   int // so oft meldet die Auftragsliste "Sitzung abgelaufen"
}

func newFakeRelay(t *testing.T) *fakeRelay {
	t.Helper()

	f := &fakeRelay{images: map[string][]byte{}, ackFails: map[string]int{}, acks: map[string]int{}}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/session", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.sessions++
		n := f.sessions
		f.mu.Unlock()

		writeJSON(w, map[string]string{"uploadUrl": fmt.Sprintf("%s/upload?u=s%d", f.srv.URL, n)})
	})
	mux.HandleFunc("GET /api/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.expire > 0 {
			f.expire--
			http.Error(w, "Sitzung abgelaufen", http.StatusUnauthorized)
			return
		}

		writeJSON(w, append([]relay.Job{}, f.pending...))
	})
	mux.HandleFunc("GET /api/v1/job/image", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		data, ok := f.images[r.URL.Query().Get("id")]
		f.mu.Unlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("DELETE /api/v1/job", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")

		f.mu.Lock()
		defer f.mu.Unlock()

		f.acks[id]++
		if f.ackFails[id] > 0 {
			f.ackFails[id]--
			http.Error(w, "kurz weg", http.StatusServiceUnavailable)
			return
		}

		for i, job := range f.pending {
			if job.ID == id {
				f.pending = append(f.pending[:i], f.pending[i+1:]...)
				break
			}
		}

		writeJSON(w, map[string]bool{"acknowledged": true})
	})

	// Jede Anfrage ohne das richtige Token wird abgewiesen, wie Nago es mit
	// einem unbekannten Token täte.
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unbekanntes Token", http.StatusBadRequest)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.srv.Close)

	return f
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeRelay) add(job relay.Job, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pending = append(f.pending, job)
	f.images[job.ID] = data
}

func (f *fakeRelay) snapshot() (pending int, sessions int, acks map[string]int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	acks = map[string]int{}
	for k, v := range f.acks {
		acks[k] = v
	}

	return len(f.pending), f.sessions, acks
}

// settings sind die Einstellungen, die der Poller bei jedem Durchlauf liest.
// Im Betrieb ändern sie sich zur Laufzeit; hier auch.
type settings struct {
	v atomic.Pointer[relay.Options]
}

func newSettings(opts relay.Options) *settings {
	s := &settings{}
	s.set(opts)
	return s
}

func (s *settings) set(opts relay.Options) { s.v.Store(&opts) }
func (s *settings) load() relay.Options    { return *s.v.Load() }

// delivery ist ein Aufruf von Deliver.
type delivery struct {
	job  relay.Job
	data []byte
	ok   bool
}

// inbox nimmt die Bilder entgegen. fail bestimmt, wie oft die Übernahme
// eines Auftrags noch scheitert – etwa weil die Karte gerade voll ist.
type inbox struct {
	mu    sync.Mutex
	calls []delivery
	fail  map[string]int
}

func (in *inbox) deliver(_ context.Context, job relay.Job, data []byte) error {
	in.mu.Lock()
	defer in.mu.Unlock()

	if in.fail[job.ID] > 0 {
		in.fail[job.ID]--
		in.calls = append(in.calls, delivery{job: job, data: data})
		return fmt.Errorf("Speicherkarte voll")
	}

	in.calls = append(in.calls, delivery{job: job, data: data, ok: true})
	return nil
}

func (in *inbox) snapshot() []delivery {
	in.mu.Lock()
	defer in.mu.Unlock()
	return append([]delivery{}, in.calls...)
}

// run startet den Poller und hält ihn bis zum Testende am Laufen. Er ist
// beendet, bevor der Server schließt – sonst liefen seine letzten Anfragen
// ins Leere und der Race-Detector hätte Recht, sich zu beschweren.
func run(t *testing.T, p *relay.Poller) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.Run(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("Zeitüberschreitung: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// settle lässt den Poller noch einige Durchläufe machen. Was danach nicht
// passiert ist, passiert auch nicht mehr.
func settle() { time.Sleep(100 * time.Millisecond) }

func countOK(calls []delivery, id string) (ok, all int) {
	for _, c := range calls {
		if c.job.ID == id {
			all++
			if c.ok {
				ok++
			}
		}
	}
	return ok, all
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("keine gültige Adresse %q: %v", raw, err)
	}
	return u
}

func sameBytes(a, b []byte) bool { return bytes.Equal(a, b) }
