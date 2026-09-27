package cfgdevice

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"path/filepath"
	"slices"
	"time"

	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/pkg/blob/fs"
	nagojson "go.wdy.de/nago/pkg/data/json"

	"github.com/worldiety/gift/asset/turbojpeg"

	"github.com/torbenschinke/eventprint/app/camera"
	"github.com/torbenschinke/eventprint/app/device"
	"github.com/torbenschinke/eventprint/app/nas"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/app/relay"
	"github.com/torbenschinke/eventprint/app/usb"
	"github.com/torbenschinke/eventprint/app/wifi"
	"github.com/torbenschinke/eventprint/pkg/facecrop"
)

// Device ist das verdrahtete Gerät.
type Device struct {
	Options Options

	Photos   photo.UseCases
	Printing printing.UseCases
	Device   device.UseCases
	Relay    relay.UseCases
	USB      usb.UseCases
	NAS      nas.UseCases
	WiFi     wifi.UseCases

	// Camera ist nil, wenn die Kamera abgeschaltet ist.
	Camera *camera.Monitor

	// Queues listet die Druckerwarteschlangen für die Einstellungen.
	Queues *printing.QueueLister

	grants device.Grants
	lock   *device.Lock
	kiosk  device.KioskStore
}

// Subject liefert das Subjekt für den, der gerade vor dem Gerät steht.
//
// Es wird bei jeder Aktion neu ermittelt und nicht gemerkt. Läuft eine
// Freischaltung der Betreuung ab, gilt im nächsten Augenblick wieder die
// Gastrolle, ohne dass die Oberfläche daran denken müsste.
func (d *Device) Subject() device.Actor {
	k, err := d.kiosk.Load()
	if err != nil {
		// Im Zweifel die engere Rolle. Ein unlesbarer Zustand darf keine
		// Mediathek öffnen.
		slog.Error("cannot read kiosk state", "err", err)
		return device.NewActor(device.RoleGuest, d.grants)
	}

	if !k.Active() {
		return device.NewActor(device.RoleOwner, d.grants)
	}

	if d.lock.Unlocked() {
		return device.NewActor(device.RoleOperator, d.grants).WithEvent(k.Event)
	}

	return device.NewActor(device.RoleGuest, d.grants).WithEvent(k.Event)
}

// Relock beendet eine Freischaltung der Betreuung sofort.
func (d *Device) Relock() { d.lock.Relock() }

// Start verdrahtet das Gerät und startet seine Hintergrundarbeiten. Sie enden
// mit ctx.
func Start(ctx context.Context, opts Options) (*Device, error) {
	settingsStore := device.NewFileSettings(filepath.Join(opts.DataDir, "settings.json"))
	kioskStore := device.NewFileKiosk(filepath.Join(opts.RuntimeDir, "kiosk.json"))

	if err := seedSettings(settingsStore, opts); err != nil {
		return nil, err
	}

	loadSettings := func() device.Settings {
		s, err := settingsStore.Load()
		if err != nil {
			slog.Error("cannot load settings", "err", err)
		}

		return s.Normalized()
	}

	photoStore, err := fs.NewBlobStore(filepath.Join(opts.DataDir, "photos", "meta"))
	if err != nil {
		return nil, fmt.Errorf("cannot open photo store: %w", err)
	}

	originals, err := photo.NewDirOriginals(filepath.Join(opts.DataDir, "photos", "originals"))
	if err != nil {
		return nil, err
	}

	photos := photo.NewUseCases(nagojson.NewSloppyJSONRepository[photo.Photo, photo.ID](photoStore), originals)

	jobStore, err := fs.NewBlobStore(filepath.Join(opts.DataDir, "printjobs"))
	if err != nil {
		return nil, fmt.Errorf("cannot open job store: %w", err)
	}

	lock := device.NewLock(time.Now)

	// devUC wird erst weiter unten gesetzt, der Druck braucht aber schon
	// jetzt den Papierzähler. Der Beobachter liest den Wert deshalb erst,
	// wenn ein Auftrag tatsächlich fertig ist.
	var devUC device.UseCases

	printer := printing.NewSettingsPrinter(func() printing.Settings { return loadSettings().Printer })
	prints := printing.NewUseCases(ctx, printing.Options{
		Repository: nagojson.NewSloppyJSONRepository[printing.Job, printing.JobID](jobStore),
		Printer:    printer,
		Locate:     photos.Locate,
		DecodeJPEGScaled: func(raw []byte, minW, minH int) (image.Image, error) {
			return turbojpeg.DecodeScaled(raw, minW, minH)
		},
		RenderOptions: func() printing.RenderOptions {
			return printing.RenderOptions{AutoCrop: loadSettings().FaceCrop, DetectFaces: facecrop.Detect}
		},
		MaxKioskCopies: func() int { return loadSettings().MaxCopies },
		Observe: func(job printing.Job) {
			if job.State != printing.StateDone {
				return
			}

			if err := devUC.ConsumePaper(permission.SU(), 1); err != nil {
				slog.Error("cannot count paper", "err", err)
			}

			if err := photos.MarkPrinted(permission.SU(), job.Photos...); err != nil {
				slog.Error("cannot mark photos printed", "err", err)
			}
		},
	})

	if enforcer, ok := printer.(printing.ErrorPolicyEnforcer); ok {
		enforcer.EnsureErrorPolicy(ctx)
	}

	d := &Device{Options: opts, Photos: photos, Printing: prints, lock: lock, kiosk: kioskStore, Queues: &printing.QueueLister{}}

	poller := relay.NewPoller(func() relay.Options {
		s := loadSettings()
		return relay.Options{URL: s.RelayURL, Token: s.RelayToken}
	}, func(ctx context.Context, job relay.Job, data []byte) error {
		return tolerateUnprinted(devUC.Intake(permission.SU(), device.IntakeCmd{Name: job.Filename, Source: photo.SourceRelay, Data: data, Template: job.Template}))
	})

	if opts.CameraDir != "" {
		d.Camera = camera.New(opts.CameraDir, func() camera.Options { return camera.Options{} }, func(name string, data []byte) error {
			return tolerateUnprinted(devUC.Intake(permission.SU(), device.IntakeCmd{Name: name, Source: photo.SourceCamera, Data: data}))
		})
	}

	devUC = device.NewUseCases(settingsStore, kioskStore, lock, probes(d, poller, loadSettings), photos.Import, prints.PrintSimple)

	d.Device = devUC
	d.Relay = relay.NewUseCases(poller, func(url, token, account string) error {
		_, err := devUC.SaveSettings(permission.SU(), func(s *device.Settings) {
			s.RelayURL, s.RelayToken, s.RelayAccount = url, token, account
		})
		return err
	})
	d.WiFi = wifi.NewUseCases()
	d.USB = usb.NewUseCases(usb.ExecRunner{}, usb.StatfsFreeSpace)
	var nasClient nas.Client = nas.NewSMB()
	if opts.NASClient != nil {
		nasClient = opts.NASClient
	}

	d.NAS = nas.NewUseCases(nasClient, func() nas.Config { return loadSettings().NAS })

	d.grants = grants()

	go poller.Run(ctx)
	if d.Camera != nil {
		go d.Camera.Run(ctx)
	}

	return d, nil
}

// tolerateUnprinted behandelt ein angenommenes, aber nicht gedrucktes Bild als
// Erfolg. Sonst holte der Upload-Dienst oder die Kamera es erneut, und das
// Foto läge doppelt in der Feier.
func tolerateUnprinted(res device.IntakeResult, err error) error {
	if err == nil {
		return nil
	}

	if res.Photo.ID != "" {
		slog.Error("incoming photo stored but not printed", "photo", string(res.Photo.ID), "err", err)
		return nil
	}

	return err
}

// seedSettings übernimmt die Vorgaben aus der Umgebung in ein Gerät, dessen
// Einstellungen die Werte noch nicht kennen.
func seedSettings(store device.SettingsStore, opts Options) error {
	s, err := store.Load()
	if err != nil {
		return err
	}

	changed := false
	if s.Printer.Queue == "" && opts.PrinterQueue != "" {
		s.Printer.Queue = opts.PrinterQueue
		changed = true
	}

	if s.RelayURL == "" && opts.RelayURL != "" {
		s.RelayURL = opts.RelayURL
		changed = true
	}

	if s.RelayToken == "" && opts.RelayToken != "" {
		s.RelayToken = opts.RelayToken
		changed = true
	}

	if s.NAS.Host == "" && opts.NAS.Host != "" {
		s.NAS = opts.NAS.Normalized()
		changed = true
	}

	if !changed {
		return nil
	}

	return store.Save(s.Normalized())
}

// grants legt die Rollen fest.
//
// Der Gast bekommt, was er für die Feier braucht, und nichts darüber hinaus:
// die Fotos der Feier sehen, ein Foto in einem Kiosk-Layout drucken, den
// QR-Code sehen, und die PIN eingeben dürfen. Alles andere – Mediathek,
// Einstellungen, USB, NAS – bleibt ihm verschlossen, weil ihm die
// Berechtigungen fehlen, nicht nur die Knöpfe.
func grants() device.Grants {
	var owner []permission.ID
	for _, perms := range [][]permission.ID{
		photo.Permissions(), printing.Permissions(), device.Permissions(), relay.Permissions(),
		usb.Permissions(), nas.Permissions(), {wifi.PermScan, wifi.PermStatus, wifi.PermConnect},
	} {
		owner = append(owner, perms...)
	}

	guest := []permission.ID{
		photo.PermFindEvent, photo.PermFindByID, photo.PermLocate,
		printing.PermPrintSimple, printing.PermPreview, printing.PermFindAllJobs, printing.PermDiagnose,
		relay.PermUploadAddress,
		device.PermCurrentKiosk, device.PermUnlock,
	}

	slices.Sort(owner)

	return device.Grants{Owner: slices.Compact(owner), Guest: guest}
}
