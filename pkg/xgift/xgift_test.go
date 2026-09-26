package xgift

import (
	"testing"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/font/inter"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/gifttest"
	"github.com/worldiety/gift/ui"
)

func TestTapGateOpensOnlyOnQuickTaps(t *testing.T) {
	now := time.Unix(0, 0)
	g := NewTapGate(3, time.Second)
	g.now = func() time.Time { return now }

	tap := func(after time.Duration) bool {
		now = now.Add(after)
		return g.Tap()
	}

	if tap(0) || tap(500*time.Millisecond) {
		t.Fatal("zu früh geöffnet")
	}

	if !tap(500 * time.Millisecond) {
		t.Fatal("drei zügige Tipps müssen öffnen")
	}

	// Vereinzelte Tipps über einen Abend summieren sich nicht.
	if tap(0) || tap(2*time.Second) || tap(2*time.Second) {
		t.Fatal("langsame Tipps dürfen nicht öffnen")
	}
}

func TestTouchSelectToggles(t *testing.T) {
	g := ui.NewGallery(asset.NewCollection([]asset.Metadata{{ID: "a"}, {ID: "b"}, {ID: "c"}}))

	sel := TouchSelect(g, nil, "a")
	sel = TouchSelect(g, sel, "c")
	if len(sel) != 2 || !g.Selection().Contains("a") || !g.Selection().Contains("c") {
		t.Fatalf("Auswahl = %v", sel)
	}

	sel = TouchSelect(g, sel, "a")
	if len(sel) != 1 || g.Selection().Contains("a") {
		t.Fatalf("erneuter Tipp muss abwählen: %v", sel)
	}
}

// TestFillAndStretchSpanTheScreen: Die Spalten einer geteilten Ansicht sind so
// hoch wie der Bildschirm, und ein dehnbares Kind darin bekommt den Rest.
func TestFillAndStretchSpanTheScreen(t *testing.T) {
	h := gifttest.New(t, gifttest.Options{
		Size: geom.Sz(800, 600),
		Font: ui.MustFont(ui.FontQuery{Family: inter.Family}),
		View: HStretch(
			Fill(ui.Box().Key("sidebar")).Width(200),
			Fill(ui.VStack(
				ui.Text("oben").Key("head"),
				ui.Box().Flex(1).Key("rest"),
			)).Flex(1),
		),
	})

	h.Find(gifttest.ByKey("sidebar")).AssertSize(geom.Sz(200, 600))

	rest := h.Find(gifttest.ByKey("rest")).Bounds()
	if rest.Size().W != 600 || rest.Max.Y != 600 || rest.Size().H < 500 {
		t.Fatalf("der dehnbare Rest ist %v", rest)
	}
}

func TestResourceDropsStaleAnswers(t *testing.T) {
	var res *Resource[int]
	h := gifttest.New(t, gifttest.Options{Root: func(ctx *gift.Context) gift.View {
		res = UseResource[int](ctx, "r")
		return ui.Box()
	}})
	Install(h.App())

	slow := make(chan struct{})
	res.Load(func() (int, error) { <-slow; return 1, nil })
	res.Load(func() (int, error) { return 2, nil })

	deadline := time.Now().Add(2 * time.Second)
	for res.Value() != 2 && time.Now().Before(deadline) {
		h.Frame()
		time.Sleep(5 * time.Millisecond)
	}

	close(slow)
	time.Sleep(20 * time.Millisecond)
	h.Frame()

	if res.Value() != 2 {
		t.Fatalf("Wert = %d, die ältere Antwort hat die neuere überschrieben", res.Value())
	}
}

func TestQRSourceIsCachedAndDecodable(t *testing.T) {
	a := QRSource("https://example.org/u?x=1")
	b := QRSource("https://example.org/u?x=1")
	if a == nil || a != b {
		t.Fatal("gleicher Text muss dieselbe Quelle liefern")
	}

	if a.Metadata().MIMEType != "image/png" {
		t.Fatalf("MIME = %q", a.Metadata().MIMEType)
	}
}
