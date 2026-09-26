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
//	go run -tags nofacecrop ./cmd/gift-app -window
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
	backend "github.com/worldiety/gift/backend/ebiten"
	"github.com/worldiety/gift/font/inter"
	"github.com/worldiety/gift/ui"

	cfgdevice "github.com/torbenschinke/eventprint/app/device/cfg"
	uidevice "github.com/torbenschinke/eventprint/app/device/ui"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

var window = flag.Bool("window", false, "im Fenster statt im Vollbild starten, zum Entwickeln")

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

	// Der Entwurf rechnet in 1280 x 720 Punkten. Auf dem 1080p-Bildschirm
	// der Box ist alles um die Hälfte größer; gift kennt nur ganzzahlige
	// Dichten, deshalb vergrößert die Oberfläche selbst.
	width, height := 1280, 720
	scale := float32(1)
	if !*window {
		scale = 1.5
		width, height = 1920, 1080
	}

	if v, err := strconv.ParseFloat(os.Getenv("EVENTPRINT_UI_SCALE"), 32); err == nil && v > 0 {
		scale = float32(v)
	}

	uidevice.SetScale(scale)

	var face *uidevice.App
	app := gift.New(gift.Options{Root: func(c *gift.Context) gift.View { return face.Root(c) }})
	face = uidevice.New(dev, app)
	xgift.Install(app)

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
		IdleTPS:    10,
		IdleFrames: 120,
		OnUpdate: func() error {
			if ctx.Err() != nil {
				return backend.Terminate
			}

			return nil
		},
	})
}
