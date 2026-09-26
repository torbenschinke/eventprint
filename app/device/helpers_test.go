package device

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.wdy.de/nago/application/permission"
)

// testGrants bildet die Verdrahtung nach: Der Besitzer darf alles in diesem
// Kontext, der Gast nur das, was vor dem Kiosk nötig ist. Kiosk starten,
// Einstellungen und PIN gehören ausdrücklich nicht dazu.
func testGrants() Grants {
	return Grants{
		Owner: Permissions(),
		Guest: []permission.ID{PermCurrentKiosk, PermUnlock},
	}
}

func owner() Actor { return NewActor(RoleOwner, testGrants()) }
func guest() Actor { return NewActor(RoleGuest, testGrants()) }

// clock ist eine Uhr, die nur weiterläuft, wenn der Test es sagt. Die
// Sperre nach Fehlversuchen und die Gültigkeit der Freischaltung sollen ohne
// echtes Warten prüfbar sein.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock {
	return &clock{t: time.Date(2026, 9, 26, 18, 30, 0, 0, time.FixedZone("CEST", 2*60*60))}
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t = c.t.Add(d)
}

// fixture ist ein Gerät auf Dateien in einem Testverzeichnis: Einstellungen
// wie unter /var/lib, Kiosk wie unter /run.
type fixture struct {
	dir      string
	settings *FileSettings
	kiosk    *FileKiosk
	lock     *Lock
	clock    *clock
	mutex    *sync.Mutex
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	dir := t.TempDir()
	c := newClock()

	return &fixture{
		dir:      dir,
		settings: NewFileSettings(filepath.Join(dir, "state", "settings.json")),
		kiosk:    NewFileKiosk(filepath.Join(dir, "run", "kiosk.json")),
		lock:     NewLock(c.Now),
		clock:    c,
		mutex:    &sync.Mutex{},
	}
}

func (f *fixture) startKiosk() StartKiosk {
	return NewStartKiosk(f.mutex, f.settings, f.kiosk, f.clock.Now)
}
