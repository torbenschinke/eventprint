// Command gift-app ist die Oberfläche des Fotodruckers auf dem Touchscreen.
//
// Sie startet immer im Heimbetrieb: Mediathek, Druck-Studio, Einstellungen.
// Aus den Einstellungen oder vom Home-Bildschirm lässt sich der Kiosk für
// eine Feier starten; er gilt bis zum nächsten Neustart des Geräts.
//
// Gezeichnet wird mit gift direkt auf die GPU, ohne Browser. Konfiguriert
// wird über die Umgebung, siehe [cfgdevice.OptionsFromEnv]. Zum Ausprobieren
// am Schreibtisch:
//
//	go run -tags nofacecrop ./cmd/gift-app -window -size 1024x600
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/asset/turbojpeg"
	backend "github.com/worldiety/gift/backend/ebiten"
	"github.com/worldiety/gift/font/inter"
	"github.com/worldiety/gift/ui"

	cfgdevice "github.com/torbenschinke/eventprint/app/device/cfg"
	uidevice "github.com/torbenschinke/eventprint/app/device/ui"
	"github.com/torbenschinke/eventprint/pkg/heif"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

var (
	window = flag.Bool("window", false, "im Fenster statt im Vollbild starten, zum Entwickeln")
	size   = flag.String("size", "", "Fenstergröße zum Entwickeln, etwa 800x480 oder 1024x600")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gift-app:", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := cfgdevice.OptionsFromEnv()
	dev, err := cfgdevice.Start(ctx, opts)
	if err != nil {
		return err
	}

	ui.SetDefaultFont(ui.MustFont(ui.FontQuery{Family: inter.Family}))

	// HEIC von iPhones: für Import und Druck über image.Decode, für die
	// Vorschaubilder über die Decoderliste von gift. Ohne libheif auf dem
	// Gerät bleibt es bei JPEG und PNG.
	// Kamerafotos über libjpeg-turbo: für Vorschaubilder verkleinert schon
	// beim Dekodieren, auf dem Pi mehrfach schneller als image/jpeg. Ohne
	// die Bibliothek bleibt es bei image/jpeg.
	if turbojpeg.Register() {
		asset.RegisterDecoder(asset.MIMEJPEG, turbojpeg.Decoder{})
	} else {
		slog.Warn("libturbojpeg nicht gefunden, Vorschaubilder werden langsamer dekodiert")
	}

	if heif.Register() {
		asset.RegisterDecoder("image/heic", heif.Decoder{})
		asset.RegisterDecoder("image/heif", heif.Decoder{})
	} else {
		slog.Warn("libheif nicht gefunden, HEIC-Fotos werden abgelehnt")
	}

	var face *uidevice.App
	app := gift.New(gift.Options{Root: func(c *gift.Context) gift.View { return face.Root(c) }})
	face = uidevice.New(dev, app)
	xgift.Install(app)

	// Die Oberfläche bemisst sich selbst nach dem Bildschirm, siehe
	// app/device/ui/display.go. Die Panelgröße in Millimetern kennt nur X11.
	if mm, ok := physicalSize(); ok {
		face.SetPhysicalSize(mm)
	}

	if v, err := strconv.ParseFloat(os.Getenv("EVENTPRINT_UI_SCALE"), 32); err == nil && v > 0 {
		face.SetScale(float32(v))
	}

	pipe := asset.NewPipeline(asset.Config{
		Deliver: app.Post,
		Disk:    asset.DiskCacheConfig{Dir: filepath.Join(opts.CacheDir, "thumbnails"), Budget: 1 << 30},
	})
	defer func() {
		pipe.Close()
		app.DrainPosts()
	}()
	ui.SetImagePipeline(pipe)

	// Die Bildschirmtastatur ist die einzige Tastatur, die es an der Box
	// gibt.
	ui.SetOnScreenKeyboard(app, true)
	face.ApplyTheme()

	width, height := 1280, 720
	if w, h, ok := parseSize(*size); ok {
		width, height = w, h
	}

	if !*window {
		eb.SetFullscreen(true)
		eb.SetCursorMode(eb.CursorModeHidden)
	}

	return backend.Run(app, backend.Config{
		Title:  "eventprint",
		Width:  width,
		Height: height,
		Logger: logger,

		// Eine Box, die den Abend über wartet, soll nicht heiß werden.
		IdleTPS:    idleTPS(),
		IdleFrames: 120,

		// Nur zeichnen, wenn sich etwas ändert. Ohne das zeichnet die Box
		// auch im Leerlauf sechzigmal in der Sekunde das ganze Bild, und im
		// geschlossenen Gehäuse wird der Pi heiß.
		DrawOnDemand: os.Getenv("EVENTPRINT_DRAW_ALWAYS") == "",

		// Direkt ins Bild statt über einen Zwischenpuffer: Auf dem Pi spart
		// das bei Full-HD ein Drittel jedes Bildes. Siehe
		// backend.Config.DirectToScreen.
		DirectToScreen: os.Getenv("EVENTPRINT_NO_DIRECT") == "",
		OnUpdate: func() error {
			if ctx.Err() != nil {
				return backend.Terminate
			}

			face.Fit(app.Viewport(), app.Density())

			return nil
		},
	})
}

// idleTPS ist die Taktrate im Leerlauf. gift liest Berührungen nur im Takt;
// bei 10 Hz vergingen bis zu 100 ms, bevor ein Knopf auf den Finger
// reagierte, und das spürt man. 30 Hz halbieren das Warten auf höchstens
// 33 ms und kosten auf dem Pi wenige Prozent eines Kerns.
// EVENTPRINT_IDLE_TPS setzt einen anderen Wert.
func idleTPS() int {
	if v, err := strconv.Atoi(os.Getenv("EVENTPRINT_IDLE_TPS")); err == nil && v > 0 {
		return v
	}

	return 30
}
