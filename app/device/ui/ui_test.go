package uidevice_test

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"testing"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/font/inter"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/gifttest"
	"github.com/worldiety/gift/ui"
	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/device"
	cfgdevice "github.com/torbenschinke/eventprint/app/device/cfg"
	uidevice "github.com/torbenschinke/eventprint/app/device/ui"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/pkg/xgift"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
	"github.com/torbenschinke/eventprint/requirements/fun/modus"
)

// box ist ein Gerät mit Oberfläche, wie es auf dem Tisch steht: frische
// Daten, kein Drucker (Testbetrieb), keine Kamera.
type box struct {
	t   *testing.T
	h   *gifttest.Harness
	dev *cfgdevice.Device
}

// newBox startet ein Gerät. Die Fotos in seed entstehen vor dem ersten Bild,
// wie Fotos, die schon vor dem Einschalten auf der Karte lagen.
func newBox(t *testing.T, seed ...photo.EventID) *box {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	dir := t.TempDir()
	dev, err := cfgdevice.Start(ctx, cfgdevice.Options{
		DataDir:    dir + "/data",
		RuntimeDir: dir + "/run",
		CacheDir:   dir + "/cache",
	})
	if err != nil {
		t.Fatal(err)
	}

	b := &box{t: t, dev: dev}
	for _, e := range seed {
		b.photo(e)
	}

	face := uidevice.New(dev, nil)
	h := gifttest.New(t, gifttest.Options{
		Root: face.Root,
		Size: geom.Sz(1280, 720),
		Font: ui.MustFont(ui.FontQuery{Family: inter.Family}),
	})
	face.SetApp(h.App())
	xgift.Install(h.App())
	b.h = h

	return b
}

// photo legt ein Foto an, als käme es aus der angegebenen Quelle.
func (b *box) photo(event photo.EventID) photo.Photo {
	b.t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 60, 40))
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		b.t.Fatal(err)
	}

	p, err := b.dev.Photos.Import(permission.SU(), photo.ImportCmd{Name: "test.jpg", Source: photo.SourceRelay, Event: event, Unseen: event == "", Data: buf.Bytes()})
	if err != nil {
		b.t.Fatal(err)
	}

	return p
}

// waitFor pumpt Bilder, bis etwas auf dem Bildschirm steht. Die Oberfläche
// lädt ihre Daten nebenläufig; ein einzelnes Settle sähe den Stand vor der
// Antwort.
func (b *box) waitFor(s gifttest.Selector) gifttest.Node {
	b.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b.h.Frame()
		if b.h.Exists(s.And(gifttest.Visible())) {
			return b.h.First(s.And(gifttest.Visible()))
		}

		time.Sleep(10 * time.Millisecond)
	}

	b.t.Fatalf("%s erscheint nicht:\n%s", s, b.h.Dump())

	return gifttest.Node{}
}

func (b *box) tap(text string) {
	b.t.Helper()
	b.waitFor(gifttest.ByText(text).And(gifttest.Interactive()).Or(gifttest.Under(gifttest.Interactive()).And(gifttest.ByText(text)))).Click()
	b.h.Frame()
}

func (b *box) role() device.Role { return b.dev.Subject().Role() }

func TestHomeModeShowsInboxAfterStart(t *testing.T) {
	b := newBox(t, "", "")

	b.waitFor(gifttest.ByText("Eingang"))
	b.waitFor(gifttest.ByText("2 neu"))

	if b.role() != device.RoleOwner {
		t.Fatalf("nach dem Start: Rolle %s, erwartet Besitzer", b.role())
	}

	spec.Verified(t, modus.RModusHeim, foto.RFotoEingang)
}

func TestKioskHidesTheLibraryFromGuests(t *testing.T) {
	b := newBox(t)
	private := b.photo("")

	b.tap("Kiosk starten")
	b.waitFor(gifttest.ByText("Kiosk starten?"))
	b.tap("Trotzdem starten")
	b.waitFor(gifttest.ByText("Tippe auf ein Foto, um es zu drucken"))

	if b.role() != device.RoleGuest {
		t.Fatalf("im Kiosk: Rolle %s, erwartet Gast", b.role())
	}

	if _, err := b.dev.Photos.FindAll(b.dev.Subject(), photo.Query{}); err == nil {
		t.Fatal("ein Gast darf die Mediathek nicht lesen")
	}

	if locs, _ := b.dev.Photos.Locate(b.dev.Subject(), private.ID); len(locs) != 0 {
		t.Fatal("ein Gast darf ein privates Foto auch über seine Kennung nicht finden")
	}

	b.h.AssertNone(gifttest.ByText("Eingang"))

	spec.Verified(t, modus.RModusKiosk, modus.RModusPrivat)
}

func TestKioskOperatorNeedsThePin(t *testing.T) {
	b := newBox(t)
	if err := b.dev.Device.SetPin(b.dev.Subject(), "135790"); err != nil {
		t.Fatal(err)
	}

	b.tap("Kiosk starten")
	b.tap("Trotzdem starten")

	qr := b.waitFor(gifttest.Where("QR-Tür", func(n gifttest.Node) bool {
		return n.Type() == "ui.Button" && n.Find(gifttest.ByText("Gerade nicht möglich")).IsVisible()
	}))

	for range 5 {
		qr.Click()
		b.h.Frame()
	}

	b.waitFor(gifttest.ByText("PIN eingeben"))
	for _, d := range "135790" {
		b.tap(string(d))
	}

	b.waitFor(gifttest.ByText("Kiosk beenden"))
	if b.role() != device.RoleOperator {
		t.Fatalf("nach der PIN: Rolle %s, erwartet Betreuung", b.role())
	}

	b.tap("Kiosk beenden")
	b.waitFor(gifttest.ByText("Eingang"))

	if b.role() != device.RoleOwner {
		t.Fatalf("nach dem Beenden: Rolle %s, erwartet Besitzer", b.role())
	}

	spec.Verified(t, modus.RModusBetreuung)
}

// TestSettingsAreChangedOnTheDevice: Die Einstellungen lassen sich am Gerät
// selbst ändern – hier der Titel der nächsten Feier über die Bildschirm-
// tastatur.
func TestSettingsAreChangedOnTheDevice(t *testing.T) {
	b := newBox(t)

	b.tap("Einstellungen")
	b.tap("Kiosk-Modus")

	field := b.waitFor(gifttest.ByType("ui.TextField"))
	field.Click()
	b.h.TypeText("Sommerfest")
	b.h.Key(gift.KeyEnter)
	b.h.Frame()

	s, err := b.dev.Device.LoadSettings(b.dev.Subject())
	if err != nil {
		t.Fatal(err)
	}

	if s.EventTitle != "Sommerfest" {
		t.Fatalf("Titel = %q, erwartet Sommerfest", s.EventTitle)
	}

	spec.Verified(t, modus.RModusEinstellungen)
}
