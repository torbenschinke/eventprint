// Package cfgphotobox verdrahtet die Fotobox mit der Nago-Anwendung.
//
// Der Einstiegspunkt [Enable] folgt dem üblichen Nago-Muster für steckbare
// Module: Er legt Repositories an, baut die Anwendungsfälle, registriert die
// Seiten und ist idempotent.
package cfgphotobox

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/worldiety/option"
	"go.wdy.de/nago/application"
	"go.wdy.de/nago/application/admin"
	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/application/role"
	"go.wdy.de/nago/application/settings"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui/form"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/photobox/cfg/camera"
	"github.com/torbenschinke/eventprint/app/photobox/cfg/remote"
	uiphotobox "github.com/torbenschinke/eventprint/app/photobox/ui"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/app/wifi"
	uiwifi "github.com/torbenschinke/eventprint/app/wifi/ui"
	"github.com/torbenschinke/eventprint/pkg/diskfree"
	"github.com/torbenschinke/eventprint/pkg/facecrop"
)

// GuestRole ist die Rolle, die jeder nicht angemeldete Besucher erhält.
//
// Auf einer Feier ist eine Anmeldung undenkbar – der Gast scannt einen Code
// und will drucken. Deshalb bekommt der anonyme Nutzer gezielt genau die
// Berechtigungen, die er dafür braucht, und keine darüber hinaus.
const GuestRole role.ID = "de.torbenschinke.eventprint.guest"

// OperatorRole ist die Rolle desjenigen, der die Fotobox betreut.
//
// Sie ist nötig, weil die Gastrolle ausschließlich für anonyme Besucher gilt:
// Wer sich anmeldet, um den Drucker einzurichten, verlöre sonst den Zugriff
// auf Historie und Druckstatus. Zusätzlich zu den Gastrechten enthält sie das
// Löschen von Fotos.
const OperatorRole role.ID = "de.torbenschinke.eventprint.operator"

// BootstrapAdminMail ist das Konto, das Nago über EnableBootstrapAdmin anlegt.
// Es erhält bewusst nur nago.*-Berechtigungen, deshalb muss ihm die
// Betreuer-Rolle ausdrücklich zugewiesen werden.
const BootstrapAdminMail user.Email = "admin@localhost"

// Options konfigurieren die Fotobox.
type Options struct {
	// EventTitle erscheint als Überschrift auf dem Startbildschirm.
	EventTitle string

	// PrinterQueue ist der Name der CUPS-Warteschlange, z. B. "CZ01".
	// Leer bedeutet Testbetrieb ohne Drucker.
	PrinterQueue string
}

// OptionsFromEnv liest die Konfiguration aus Umgebungsvariablen. Damit lässt
// sich dieselbe Binärdatei ohne Neuübersetzung an unterschiedliche
// Veranstaltungen anpassen.
//
//	EVENTPRINT_TITLE          Überschrift, z. B. "Hochzeit Anna & Ben"
//	EVENTPRINT_PRINTER        CUPS-Warteschlange, leer = Testbetrieb
func OptionsFromEnv() Options {
	return Options{
		EventTitle:   os.Getenv("EVENTPRINT_TITLE"),
		PrinterQueue: os.Getenv("EVENTPRINT_PRINTER"),
	}
}

// Management ist das installierte Fotobox-Modul.
type Management struct {
	Photos   photo.UseCases
	Printing printing.UseCases
	Pages    uiphotobox.Pages
}

// Enable installiert die Fotobox in der übergebenen Konfiguration.
func Enable(cfg *application.Configurator, opts Options) (Management, error) {
	if management, ok := core.FromContext[Management](cfg.Context(), ""); ok {
		return management, nil
	}

	images, err := cfg.ImageManagement()
	if err != nil {
		return Management{}, err
	}

	photoRepo, err := application.JSONRepository[photo.Photo, photo.ID](cfg, "eventprint.photo")
	if err != nil {
		return Management{}, err
	}

	jobRepo, err := application.JSONRepository[printing.Job, printing.JobID](cfg, "eventprint.printjob")
	if err != nil {
		return Management{}, err
	}

	settingsMgmt, err := cfg.SettingsManagement()
	if err != nil {
		return Management{}, err
	}

	users, err := cfg.UserManagement()
	if err != nil {
		return Management{}, err
	}

	sessions, err := cfg.SessionManagement()
	if err != nil {
		return Management{}, err
	}

	loadPrinterSettings := func() printing.Settings {
		return settings.ReadGlobal[printing.Settings](settingsMgmt.UseCases.LoadGlobal)
	}

	loadBoothSettings := func() Settings {
		return settings.ReadGlobal[Settings](settingsMgmt.UseCases.LoadGlobal)
	}

	if err := applyPrinterDefaults(settingsMgmt, opts, loadPrinterSettings); err != nil {
		return Management{}, err
	}

	if err := applyBoothDefaults(settingsMgmt, opts, loadBoothSettings); err != nil {
		return Management{}, err
	}
	if err := applyCameraDefaults(settingsMgmt, loadBoothSettings); err != nil {
		return Management{}, err
	}
	if err := applyAutoCropDefaults(settingsMgmt, loadBoothSettings); err != nil {
		return Management{}, err
	}

	// Die Auswahlliste der Warteschlangen für das Einstellungsformular.
	queues := &printing.QueueLister{}
	cfg.AddContextValue(core.ContextValue[form.Source](printing.PrinterSource, cupsQueueSource{ctx: cfg.Context(), queues: queues}))
	cfg.AddContextValue(core.ContextValue[form.Source](TemplateSource, templateSource{}))

	warnAboutMissingQueue(cfg.Context(), queues, loadPrinterSettings())

	// Das Archiv sammelt jedes eingehende Bild im Original, damit die Fotos
	// nach der Feier digital weitergegeben werden können. Es wird nur
	// beschrieben – gelöscht wird dort nichts, auch nicht, wenn ein Foto aus
	// der Historie verschwindet.
	archiveDir := filepath.Join(cfg.DataDir(), "photos", "originals")

	archive, err := photo.NewDirArchive(archiveDir)
	if err != nil {
		return Management{}, err
	}

	slog.Info("photo archive ready", "dir", archiveDir)

	imgStores, err := newImageStores(cfg)
	if err != nil {
		return Management{}, err
	}

	photos := photo.NewUseCases(cfg.EventBus(), photoRepo, images.UseCases, archive, archiveDir, imgStores.Purge)

	printer := printing.NewSettingsPrinter(loadPrinterSettings)

	prints := printing.NewUseCases(cfg.Context(), cfg.EventBus(), jobRepo,
		printer, photos.OpenOriginal, func() printing.RenderOptions {
			return printing.RenderOptions{
				AutoCrop:    loadBoothSettings().AutoCrop,
				DetectFaces: facecrop.Detect,
			}
		})

	// Die Fehlerbehandlung der Warteschlange gleich beim Start absichern,
	// damit ein fehlendes Recht im Protokoll steht, bevor der erste Gast da
	// ist – und nicht erst beim ersten Auftrag mitten auf der Feier.
	//
	// Wird die Warteschlange später umgestellt, holt der Drucker das vor dem
	// ersten Auftrag an das neue Ziel selbst nach.
	if enforcer, ok := printer.(printing.ErrorPolicyEnforcer); ok {
		enforcer.EnsureErrorPolicy(cfg.Context())
	}

	relay := remote.NewManager(func() remote.Options {
		booth := loadBoothSettings()
		return remote.Options{URL: booth.UploaderURL, Token: booth.UploaderToken, Interval: 10 * time.Second}
	}, photos, prints)
	go relay.Run(cfg.Context())

	pages := uiphotobox.Pages{
		Booth:   ".",
		Upload:  "upload",
		Gallery: "gallery",
		Jobs:    "print/status",
		WiFi:    "settings/wifi",
		Storage: "settings/storage",
	}

	// Freischaltungen und Berührungszähler leben nur im Speicher. Ein Neustart
	// sperrt damit jede offene Sitzung wieder zu - das ist die gewünschte
	// Richtung, denn danach weiß niemand mehr, wer vor dem Bildschirm steht.
	pinLock := NewPinLock()
	tapGates := newTapGates()

	operatorUID, err := ensureOperatorUser(users)
	if err != nil {
		return Management{}, err
	}

	loadPin := func() PinHash {
		return loadBoothSettings().Pin()
	}

	storePin := func(h PinHash) error {
		return settingsMgmt.UseCases.StoreGlobal(user.SU(), loadBoothSettings().WithPin(h))
	}

	// Ein eigener Beobachter ist nicht mehr nötig: Nago stellt die Sitzung bei
	// jedem neuen Fenster selbst wieder her, weil die Anmeldung im
	// Sitzungsspeicher steht und nicht bloß im Arbeitsspeicher dieser
	// Anwendung.

	// signIn meldet die Sitzung als Betreuer an.
	//
	// Beide Schritte sind nötig: LoginUser schreibt die Anmeldung in die
	// Sitzung und überlebt damit einen Seitenwechsel; UpdateSubject wirkt
	// sofort im laufenden Fenster, ohne das die Seite erst nach einem Neuladen
	// die neuen Rechte hätte.
	signIn := func(wnd core.Window) error {
		if err := sessions.UseCases.LoginUser(wnd.Session().ID(), operatorUID); err != nil {
			return fmt.Errorf("cannot sign in the operator: %w", err)
		}

		optSubject, err := users.UseCases.SubjectFromUser(user.SU(), operatorUID)
		if err != nil {
			return fmt.Errorf("cannot build the operator subject: %w", err)
		}

		if optSubject.IsSome() {
			wnd.UpdateSubject(optSubject.Unwrap())
		}

		return nil
	}

	// boothUploadURL bildet die Adresse, die im QR-Code landet.
	//
	// Die Reihenfolge ist die der Verlässlichkeit:
	//
	//  1. die von Hand gesetzte öffentliche Adresse,
	//  2. die Adresse der Fotobox im örtlichen Netz,
	//  3. das, was Nago aus der ersten Verbindung ableitet.
	//
	// Der dritte Fall ist der gefährliche: Auf dem Kiosk verbindet sich der
	// eigene Browser als Erster, und Nago leitet daraus localhost ab. Im
	// QR-Code stand dann eine Adresse, die kein Gast erreicht - und der Code
	// sah aus wie jeder andere.
	//
	// Schritt 2 repariert genau das ohne Zutun: Die Fotobox steht mit den
	// Gästen im selben WLAN, und deren Handys erreichen sie unter dieser
	// Adresse. Sie von Hand einzutragen wäre ein Schritt, den beim Aufbau
	// niemand mehr weiß.
	boothUploadURL := func() string {
		derived := loadBoothSettings().PublicURLFor(string(pages.Upload), func() string {
			return cfg.ContextPathURI(string(pages.Upload), nil)
		})

		return withLANHost(derived, lanAddress(net.Interfaces))
	}

	cam := camera.New(filepath.Join(cfg.DataDir(), "camera", "incoming"), func() camera.Options {
		booth := loadBoothSettings()
		return camera.Options{
			AutoPrint:         booth.CameraAutoPrint,
			AutoPrintTemplate: booth.CameraTemplate,
			ScanInterval:      time.Second,
			DetectInterval:    10 * time.Second,
		}
	}, photos, prints)
	go cam.Run(cfg.Context())

	uiOpts := uiphotobox.Options{
		Pages:      pages,
		Photos:     photos,
		Printing:   prints,
		ArchiveDir: archiveDir,
		EventTitle: func() string {
			return loadBoothSettings().EventTitle
		},
		PrinterSettings: settingsMgmt.Pages.PageSettings,
		PrinterSettingsParams: core.Values{
			"type": settings.TypeIdent(reflect.TypeFor[printing.Settings]()),
		},
		BoothSettings: settingsMgmt.Pages.PageSettings,
		BoothSettingsParams: core.Values{
			"type": settings.TypeIdent(reflect.TypeFor[Settings]()),
		},
		CanConfigure: func(wnd core.Window) bool {
			return wnd.Subject().HasPermission(PermConfigure)
		},
		Pin: uiphotobox.PinAccess{
			Configured: func() bool {
				return loadPin().Configured()
			},
			Verify: func(wnd core.Window, pin string) error {
				if err := pinLock.Verify(string(wnd.Session().ID()), pin, loadPin()); err != nil {
					return err
				}

				return signIn(wnd)
			},
			Configure: func(wnd core.Window, pin string) error {
				next, err := pinLock.Configure(string(wnd.Session().ID()), pin, loadPin())
				if err != nil {
					return err
				}

				if err := storePin(next); err != nil {
					return err
				}

				// Wer die PIN gerade vergeben hat, ist damit angemeldet.
				return signIn(wnd)
			},
			Tap: func(sessionID string) bool {
				return tapGates.Tap(sessionID)
			},
		},
		// UploadProblem uebersetzt den Zustand des Relais in einen Satz, der
		// vor dem Geraet stehen darf.
		//
		// Die localhost-Adresse ist dabei der heimtueckischste Fall: Nago
		// leitet die eigene Adresse aus der ersten Verbindung ab, und auf dem
		// Kiosk ist das der Rechner selbst. Der QR-Code sieht dann normal aus
		// und fuehrt ins Nichts.
		UploadProblem: func() string {
			switch relay.State() {
			case remote.StateMissingToken:
				return "Der Upload-Dienst ist eingetragen, aber ohne Zugangstoken. Als Betreuer anmelden und es nachtragen."
			case remote.StateConnecting:
				return "Der Upload-Dienst antwortet nicht. Meist stimmt das Zugangstoken nicht oder es fehlt die Rolle Fotobox-Relay."
			}

			// Ohne Upload-Dienst laden Gaeste direkt bei der Fotobox hoch.
			// Dann muss aber die eigene Adresse von aussen erreichbar sein.
			if isLocalOnly(boothUploadURL()) {
				return "Die Fotobox ist in keinem Netz erreichbar. WLAN verbinden oder als Betreuer die öffentliche Adresse setzen."
			}

			return ""
		},
		UploadURL: func() string {
			if relay.UploadURL() != "" {
				return relay.UploadURL()
			}

			return boothUploadURL()
		},

		// CameraStatus bringt den Zustand der Kamera auf den Startbildschirm.
		// Bricht das Tethering ab, sagte das vorher nur das Log, und die
		// Fotobox nahm scheinbar weiter Bilder auf.
		CameraStatus: cam.Status,
	}

	// Der Touchscreen der Fotobox misst 1024x600. Auf so wenig Hoehe ist jede
	// Zeile, die nicht zur Sache gehoert, verlorener Platz: Der Fusszeile mit
	// Impressum und Nutzungsbedingungen steht auf einem Geraet, das den Abend
	// ueber Fotos zeigt, niemand gegenueber. Sie bleibt auf der Upload-Seite,
	// denn die laeuft auf dem Smartphone eines Gastes.
	//
	// BodyFullSize dazu: Nago begrenzt den Inhalt sonst auf eine lesbare
	// Spaltenbreite. Das ist bei Fliesstext richtig und bei einem Bilderraster
	// auf einem kleinen Bildschirm verschenkte Flaeche.
	cfg.NoFooter(pages.Booth, pages.Gallery, pages.Jobs)
	cfg.BodyFullSize(pages.Booth, pages.Gallery, pages.Jobs)

	// Der Startbildschirm bekommt für Gäste GAR KEINEN Rahmen.
	//
	// Die Menüeinträge auf eine Berechtigung zu stellen genügte nicht: Nago
	// zeichnet die Leiste auch dann, wenn kein einziger Eintrag übrig bleibt.
	// Übrig blieb ein leerer Balken, der auf 1024x600 Platz kostet und Gäste
	// zum Suchen einlädt.
	//
	// Für die Veranstaltung reicht die Seite selbst: Fotos ansehen, antippen,
	// drucken. Erst wer sich mit der PIN angemeldet hat, bekommt den Rahmen
	// mit Menü und Admin-Center.
	// boothPage haengt den Rahmen nur fuer die Betreuung an.
	//
	// Fuer Gaeste bleibt die nackte Seite ueber die volle Breite. Die
	// Menueeintraege auf eine Berechtigung zu stellen genuegte nicht: Nago
	// zeichnet die Leiste auch dann, wenn kein einziger Eintrag uebrig bleibt,
	// und uebrig blieb ein leerer Balken.
	boothPage := func(page func(wnd core.Window) core.View) func(wnd core.Window) core.View {
		decorated := cfg.DecorateRootView(page)

		return func(wnd core.Window) core.View {
			if wnd.Subject().HasPermission(PermConfigure) {
				return decorated(wnd)
			}

			return page(wnd)
		}
	}

	cfg.RootView(pages.Booth, boothPage(func(wnd core.Window) core.View {
		return uiphotobox.PageBooth(wnd, uiOpts)
	}))

	cfg.RootView(pages.Gallery, boothPage(func(wnd core.Window) core.View {
		return uiphotobox.PageGallery(wnd, uiOpts)
	}))

	wifiUseCases := wifi.NewUseCases()

	cfg.RootViewWithDecoration(pages.WiFi, func(wnd core.Window) core.View {
		return uiwifi.PageWiFi(wnd, uiwifi.Options{WiFi: wifiUseCases})
	})

	storageOpts := uiphotobox.StorageOptions{
		Photos: photos,
		DiskUsage: func() (diskfree.Usage, error) {
			return diskfree.Of(cfg.DataDir())
		},
		ImageBytes: imgStores.Bytes,
	}

	cfg.RootViewWithDecoration(pages.Storage, func(wnd core.Window) core.View {
		return uiphotobox.PageStorage(wnd, storageOpts)
	})

	// Die Karte erscheint im Admin-Center nur fuer Traeger der Berechtigung,
	// also nach der PIN. Nago blendet sie sonst selbst aus.
	cfg.AddAdminCenterGroup(func(subject auth.Subject) admin.Group {
		return admin.Group{
			Title: "Fotobox",
			Entries: []admin.Card{
				{
					Title:      "Funkverbindung",
					Text:       "Das Funknetz vor Ort auswaehlen und die Fotobox damit verbinden.",
					Target:     pages.WiFi,
					Permission: wifi.PermConnect,
				},
				{
					Title:      "Speicher und Archiv",
					Text:       "Nach der Veranstaltung: Fotos als ZIP herunterladen und die Fotobox freiraeumen.",
					Target:     pages.Storage,
					Permission: photo.PermInspectArchive,
				},
			},
		}
	})

	cfg.RootView(pages.Jobs, boothPage(func(wnd core.Window) core.View {
		return uiphotobox.PageJobs(wnd, uiOpts)
	}))

	// Die Upload-Seite bewusst ohne Scaffold: Auf dem Smartphone eines Gastes
	// stört ein Menü nur, er soll genau eine Sache tun können.
	cfg.RootView(pages.Upload, func(wnd core.Window) core.View {
		return uiphotobox.PageUpload(wnd, uiOpts)
	})

	if err := enableGuestAccess(cfg); err != nil {
		return Management{}, err
	}

	cfg.AddAdminCenterGroup(func(subject auth.Subject) admin.Group {
		return admin.Group{
			Title: "Fotobox",
			Entries: []admin.Card{
				{
					Title:      "Foto-Historie",
					Text:       "Alle Fotos der Veranstaltung ansehen, nachdrucken oder löschen.",
					Target:     pages.Gallery,
					Permission: photo.PermFindAll,
				},
				{
					Title:      "Druckstatus",
					Text:       "Warteschlange des Fotodruckers einsehen und fehlgeschlagene Aufträge wiederholen.",
					Target:     pages.Jobs,
					Permission: printing.PermFindAllJobs,
				},
			},
		}
	})

	management := Management{
		Photos:   photos,
		Printing: prints,
		Pages:    pages,
	}

	cfg.AddContextValue(core.ContextValue("eventprint.photobox", management))

	slog.Info("installed photo booth", "printer", prints.Printer.Name())

	return management, nil
}

// applyPrinterDefaults schreibt die Umgebungsvariable einmalig in die
// Einstellungen.
//
// Damit bleibt der unbeaufsichtigte Betrieb per systemd oder Container
// möglich, ohne dass die Umgebung die Oberfläche dauerhaft überstimmt: Sobald
// eine Warteschlange gespeichert ist, hat die Einstellung Vorrang.
func applyPrinterDefaults(mgmt application.SettingsManagement, opts Options, load func() printing.Settings) error {
	next, changed := withPrinterDefault(opts, load())
	if !changed {
		return nil
	}

	if err := mgmt.UseCases.StoreGlobal(user.SU(), next); err != nil {
		return fmt.Errorf("cannot apply printer default: %w", err)
	}

	slog.Info("applied printer queue from environment", "queue", next.Queue)

	return nil
}

// withPrinterDefault übernimmt die Warteschlange aus der Umgebung, solange in
// den Einstellungen noch keine steht.
//
// Die Entscheidung steht getrennt vom Speichern, damit sie ohne eine laufende
// Nago-Anwendung prüfbar ist. Sie ist der Teil, der falsch sein kann; das
// Schreiben ist es nicht.
func withPrinterDefault(opts Options, current printing.Settings) (printing.Settings, bool) {
	queue := strings.TrimSpace(opts.PrinterQueue)
	if queue == "" || current.Queue != "" {
		return current, false
	}

	current.Queue = queue

	return current, true
}

// applyBoothDefaults übernimmt den Veranstaltungstitel aus der Umgebung.
// Siehe [applyPrinterDefaults].
func applyBoothDefaults(mgmt application.SettingsManagement, opts Options, load func() Settings) error {
	next, changed := withBoothDefault(opts, load())
	if !changed {
		return nil
	}

	if err := mgmt.UseCases.StoreGlobal(user.SU(), next); err != nil {
		return fmt.Errorf("cannot apply event title default: %w", err)
	}

	return nil
}

// withBoothDefault übernimmt den Titel aus der Umgebung, solange keiner steht.
func withBoothDefault(opts Options, current Settings) (Settings, bool) {
	title := strings.TrimSpace(opts.EventTitle)
	if title == "" || current.EventTitle != "" {
		return current, false
	}

	current.EventTitle = title

	return current, true
}

func applyCameraDefaults(mgmt application.SettingsManagement, load func() Settings) error {
	next, changed := withCameraDefaults(load())
	if !changed {
		return nil
	}

	if err := mgmt.UseCases.StoreGlobal(user.SU(), next); err != nil {
		return fmt.Errorf("cannot apply camera defaults: %w", err)
	}

	return nil
}

// withCameraDefaults setzt die Erstbelegung der Kamera.
//
// Der Merker verhindert, dass eine bewusst abgeschaltete Automatik beim
// nächsten Start wieder anspringt. Ohne ihn wäre "aus" nicht von "noch nie
// eingestellt" zu unterscheiden.
func withCameraDefaults(current Settings) (Settings, bool) {
	if current.CameraDefaultsApplied {
		return current, false
	}

	current.CameraAutoPrint = true
	current.CameraTemplate = printing.TemplatePolaroid
	current.CameraDefaultsApplied = true

	return current, true
}

func applyAutoCropDefaults(mgmt application.SettingsManagement, load func() Settings) error {
	next, changed := withAutoCropDefaults(load())
	if !changed {
		return nil
	}

	if err := mgmt.UseCases.StoreGlobal(user.SU(), next); err != nil {
		return fmt.Errorf("cannot apply auto-crop defaults: %w", err)
	}

	return nil
}

// withAutoCropDefaults setzt die Erstbelegung des Bildausschnitts.
// Siehe [withCameraDefaults] zum Merker.
func withAutoCropDefaults(current Settings) (Settings, bool) {
	if current.AutoCropDefaultsApplied {
		return current, false
	}

	current.AutoCrop = true
	current.AutoCropDefaultsApplied = true

	return current, true
}

// warnAboutMissingQueue meldet beim Start, wenn die eingestellte
// Warteschlange gar nicht existiert. Ohne diese Prüfung fällt der Tippfehler
// erst beim ersten Druckversuch auf – im Zweifel mitten auf der Feier.
func warnAboutMissingQueue(ctx context.Context, queues *printing.QueueLister, cfg printing.Settings) {
	if cfg.TestMode() {
		slog.Warn("no printer queue configured, running in test mode")
		return
	}

	if queues.Exists(ctx, cfg.Queue) {
		slog.Info("printing to CUPS queue", "queue", cfg.Queue)
		return
	}

	available, _ := queues.List(ctx)
	slog.Warn("configured printer queue does not exist", "queue", cfg.Queue, "available", available)
}

// cupsQueueSource stellt die CUPS-Warteschlangen als Auswahlliste für das
// Einstellungsformular bereit. Ohne sie wäre der Warteschlangenname ein
// Freitextfeld – und ein Tippfehler fiele erst beim ersten Druck auf.
type cupsQueueSource struct {
	ctx    context.Context
	queues *printing.QueueLister
}

type templateSource struct{}

func (templateSource) FindAll(auth.Subject) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for _, tpl := range printing.Templates() {
			if !yield(string(tpl.ID), nil) {
				return
			}
		}
	}
}

func (templateSource) FindByID(_ auth.Subject, id string) (option.Opt[form.Entity], error) {
	tpl := printing.TemplateByID(printing.TemplateID(id))
	if string(tpl.ID) != id {
		return option.None[form.Entity](), nil
	}
	return option.Some(form.Entity{ID: id, Value: tpl.Name}), nil
}

func (s cupsQueueSource) FindAll(subject auth.Subject) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		names, err := s.queues.List(s.ctx)
		if err != nil {
			// Fehlt lpstat, bleibt die Liste leer; der Name lässt sich dann
			// von Hand eintragen. Ein harter Fehler wäre hier unangemessen.
			slog.Warn("cannot list CUPS queues", "err", err)
			return
		}

		for _, name := range names {
			if !yield(name, nil) {
				return
			}
		}
	}
}

func (s cupsQueueSource) FindByID(subject auth.Subject, id string) (option.Opt[form.Entity], error) {
	return option.Some(form.Entity{ID: id, Value: id}), nil
}

// enableGuestAccess legt Gast- und Betreuerrolle an und verteilt sie.
func enableGuestAccess(cfg *application.Configurator) error {
	roles, err := cfg.RoleManagement()
	if err != nil {
		return err
	}

	if _, err := roles.UseCases.Upsert(user.SU(), role.Role{
		ID:          GuestRole,
		Name:        "Fotobox-Gast",
		Description: "Darf Fotos hochladen, ansehen und drucken – ohne Anmeldung.",
	}); err != nil {
		return err
	}

	if err := roles.UseCases.UpdatePermissions(user.SU(), GuestRole, guestPermissions()); err != nil {
		return err
	}

	if _, err := roles.UseCases.Upsert(user.SU(), role.Role{
		ID:          OperatorRole,
		Name:        "Fotobox-Betreuer",
		Description: "Darf zusätzlich zu den Gastrechten Fotos löschen und die Fotobox einrichten.",
	}); err != nil {
		return err
	}

	if err := roles.UseCases.UpdatePermissions(user.SU(), OperatorRole, operatorPermissions()); err != nil {
		return err
	}

	settingsMgmt, err := cfg.SettingsManagement()
	if err != nil {
		return err
	}

	usrSettings := settings.ReadGlobal[user.Settings](settingsMgmt.UseCases.LoadGlobal)
	if !containsRole(usrSettings.AnonRoles, GuestRole) {
		usrSettings.AnonRoles = append(usrSettings.AnonRoles, GuestRole)
		if err := settingsMgmt.UseCases.StoreGlobal(user.SU(), usrSettings); err != nil {
			return err
		}
	}

	return grantOperatorToBootstrapAdmin(cfg)
}

// grantOperatorToBootstrapAdmin weist dem Bootstrap-Administrator die
// Betreuer-Rolle zu.
//
// Ohne diesen Schritt zeigt die Anwendung nach dem Anmelden auf jeder Seite
// "Zugriff verweigert": Nago vergibt an dieses Konto absichtlich nur
// nago.*-Berechtigungen, damit ein Administrator nicht automatisch in die
// Fachdaten der Anwendung sehen kann. Für eine Fotobox, die genau eine
// Aufgabe hat, ist diese Trennung unnötig – wer sie einrichtet, betreut sie
// auch.
func grantOperatorToBootstrapAdmin(cfg *application.Configurator) error {
	users, err := cfg.UserManagement()
	if err != nil {
		return err
	}

	optUsr, err := users.UseCases.FindByMail(user.SU(), BootstrapAdminMail)
	if err != nil {
		return fmt.Errorf("cannot find bootstrap admin: %w", err)
	}

	// Kein Bootstrap-Admin eingerichtet: dann vergibt jemand anderes die
	// Rolle über die Nutzerverwaltung.
	if optUsr.IsNone() {
		return nil
	}

	usr := optUsr.Unwrap()

	var roleIDs []role.ID
	for id, err := range users.UseCases.ListRoles(user.SU(), usr.ID) {
		if err != nil {
			return fmt.Errorf("cannot list roles of bootstrap admin: %w", err)
		}

		roleIDs = append(roleIDs, id)
	}

	if containsRole(roleIDs, OperatorRole) {
		return nil
	}

	roleIDs = append(roleIDs, OperatorRole)
	if err := users.UseCases.UpdateOtherRoles(user.SU(), usr.ID, roleIDs); err != nil {
		return fmt.Errorf("cannot grant operator role: %w", err)
	}

	slog.Info("granted photo booth operator role", "user", string(BootstrapAdminMail))

	return nil
}

// guestPermissions ist die vollständige Liste dessen, was ein Gast darf.
// Insbesondere fehlt hier das Löschen von Fotos – das bleibt dem angemeldeten
// Betreuer vorbehalten.
//
// Wer hier eine Berechtigung vergisst, merkt es nicht am Quelltext: Die
// betroffene Seite bleibt einfach leer. Jede neue Berechtigung gehört deshalb
// entweder in diese Liste oder mit Begründung in die des Betreuers.
func guestPermissions() []permission.ID {
	return []permission.ID{
		photo.PermImport,
		photo.PermFindByID,
		photo.PermFindAll,

		// Der Startbildschirm zeigt die jüngsten Bilder.
		photo.PermFindLatest,

		// Die Vorschau im Druckdialog liest dafür die Originaldaten.
		photo.PermOpenOriginal,

		printing.PermPrint,
		printing.PermPreview,
		printing.PermFindAllJobs,
		printing.PermFindJobByID,

		// Die Druckstatus-Seite nennt den Zustand des Druckers, etwa ein
		// leeres Papierfach. Ohne das steht dort ein Gast vor einer Liste,
		// die sich nicht bewegt, und erfährt nicht warum.
		printing.PermDiagnose,
	}
}

// operatorPermissions sind die Rechte der Bedienung: alles, was ein Gast
// darf, zuzüglich des Löschens, des Wiederholens von Druckaufträgen und des
// Einrichtens.
func operatorPermissions() []permission.ID {
	return append(guestPermissions(),
		photo.PermDelete,
		printing.PermRetry,

		// Erst diese Berechtigung öffnet die Einstellungen in dieser Anwendung.
		PermConfigure,

		// Und diese beiden die Einstellungsseiten von Nago selbst. Ohne sie
		// öffnet sich die Seite zwar, aber das Speichern scheitert mit einer
		// Rechteverletzung: Nagos Anwendungsfälle prüfen ihre eigenen
		// Berechtigungen, nicht unsere.
		settings.PermLoadGlobal,
		settings.PermStoreGlobal,

		// Die Fotobox wird an fremden Orten aufgebaut, das Funknetz ist jedes
		// Mal ein anderes. Ein Gast darf es nicht wechseln koennen: Das naehme
		// der Box mitten auf der Feier die Verbindung.
		wifi.PermScan,
		wifi.PermStatus,
		wifi.PermConnect,

		// Der Abbau nach der Feier: Bilder herunterladen, Karte leeren.
		// PermPurgeEvent loescht endgueltig und gehoert nie einem Gast.
		photo.PermInspectArchive,
		photo.PermExportArchive,
		photo.PermPurgeEvent,
	)
}

func containsRole(roles []role.ID, id role.ID) bool {
	for _, r := range roles {
		if r == id {
			return true
		}
	}

	return false
}
