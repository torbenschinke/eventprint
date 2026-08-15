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
	"os"
	"reflect"
	"strconv"
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

	"github.com/torbenschinke/eventprint/camera"
	"github.com/torbenschinke/eventprint/photo"
	"github.com/torbenschinke/eventprint/printing"
	"github.com/torbenschinke/eventprint/remote"
	uiphotobox "github.com/torbenschinke/eventprint/ui"
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

	// Camera konfiguriert die Übernahme der Kamerabilder.
	Camera camera.Options

	// Relay connects this private photobox to the public photoupld service.
	Relay remote.Options
}

// OptionsFromEnv liest die Konfiguration aus Umgebungsvariablen. Damit lässt
// sich dieselbe Binärdatei ohne Neuübersetzung an unterschiedliche
// Veranstaltungen anpassen.
//
//	EVENTPRINT_TITLE          Überschrift, z. B. "Hochzeit Anna & Ben"
//	EVENTPRINT_PRINTER        CUPS-Warteschlange, leer = Testbetrieb
//	EVENTPRINT_CAMERA_DIR     Tethering-Verzeichnis der Kamera
//	EVENTPRINT_CAMERA_AUTOPRINT  "true" druckt jede Aufnahme sofort
//	EVENTPRINT_CAMERA_DELETE  "true" löscht die Datei nach der Übernahme
//	EVENTPRINT_UPLD_URL        öffentliche Basis-URL von photoupld
//	EVENTPRINT_UPLD_TOKEN      Bearer-Token mit der Fotobox-Relay-Rolle
func OptionsFromEnv() Options {
	return Options{
		EventTitle:   os.Getenv("EVENTPRINT_TITLE"),
		PrinterQueue: os.Getenv("EVENTPRINT_PRINTER"),
		Camera: camera.Options{
			Dir:               os.Getenv("EVENTPRINT_CAMERA_DIR"),
			AutoPrint:         envBool("EVENTPRINT_CAMERA_AUTOPRINT"),
			AutoPrintTemplate: printing.TemplateFull,
			Delete:            envBool("EVENTPRINT_CAMERA_DELETE"),
			Interval:          time.Second,
		},
		Relay: remote.Options{
			URL:      os.Getenv("EVENTPRINT_UPLD_URL"),
			Token:    os.Getenv("EVENTPRINT_UPLD_TOKEN"),
			Interval: 10 * time.Second,
		},
	}
}

func envBool(key string) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	return err == nil && v
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

	// Die Auswahlliste der Warteschlangen für das Einstellungsformular.
	queues := &printing.QueueLister{}
	cfg.AddContextValue(core.ContextValue[form.Source](printing.PrinterSource, cupsQueueSource{ctx: cfg.Context(), queues: queues}))

	warnAboutMissingQueue(cfg.Context(), queues, loadPrinterSettings())

	photos := photo.NewUseCases(cfg.EventBus(), photoRepo, images.UseCases)
	prints := printing.NewUseCases(cfg.Context(), cfg.EventBus(), jobRepo,
		printing.NewSettingsPrinter(loadPrinterSettings), photos.OpenOriginal)

	var relay *remote.Relay
	if opts.Relay.Enabled() {
		relay, err = remote.New(opts.Relay, photos, prints)
		if err != nil {
			return Management{}, fmt.Errorf("cannot configure upload relay: %w", err)
		}
		go relay.Run(cfg.Context())
	}

	pages := uiphotobox.Pages{
		Booth:   ".",
		Upload:  "upload",
		Gallery: "gallery",
		Jobs:    "print/status",
	}

	uiOpts := uiphotobox.Options{
		Pages:    pages,
		Photos:   photos,
		Printing: prints,
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
		UploadURL: func() string {
			if relay != nil && relay.UploadURL() != "" {
				return relay.UploadURL()
			}
			// Erst zur Laufzeit auflösen: Die eingestellte öffentliche
			// Adresse kann sich jederzeit ändern, und die automatische
			// Ermittlung steht beim Konfigurieren noch nicht fest.
			return loadBoothSettings().PublicURLFor(string(pages.Upload), func() string {
				return cfg.ContextPathURI(string(pages.Upload), nil)
			})
		},
	}

	cfg.RootViewWithDecoration(pages.Booth, func(wnd core.Window) core.View {
		return uiphotobox.PageBooth(wnd, uiOpts)
	})

	cfg.RootViewWithDecoration(pages.Gallery, func(wnd core.Window) core.View {
		return uiphotobox.PageGallery(wnd, uiOpts)
	})

	cfg.RootViewWithDecoration(pages.Jobs, func(wnd core.Window) core.View {
		return uiphotobox.PageJobs(wnd, uiOpts)
	})

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

	go camera.Watch(cfg.Context(), opts.Camera, photos, prints)

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
	queue := strings.TrimSpace(opts.PrinterQueue)
	if queue == "" {
		return nil
	}

	current := load()
	if current.Queue != "" {
		return nil
	}

	current.Queue = queue
	if err := mgmt.UseCases.StoreGlobal(user.SU(), current); err != nil {
		return fmt.Errorf("cannot apply printer default: %w", err)
	}

	slog.Info("applied printer queue from environment", "queue", queue)

	return nil
}

// applyBoothDefaults übernimmt den Veranstaltungstitel aus der Umgebung, so
// lange in den Einstellungen noch keiner steht. Siehe [applyPrinterDefaults].
func applyBoothDefaults(mgmt application.SettingsManagement, opts Options, load func() Settings) error {
	title := strings.TrimSpace(opts.EventTitle)
	if title == "" {
		return nil
	}

	current := load()
	if current.EventTitle != "" {
		return nil
	}

	current.EventTitle = title
	if err := mgmt.UseCases.StoreGlobal(user.SU(), current); err != nil {
		return fmt.Errorf("cannot apply event title default: %w", err)
	}

	return nil
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
func guestPermissions() []permission.ID {
	return []permission.ID{
		photo.PermImport,
		photo.PermFindByID,
		photo.PermFindAll,
		printing.PermPrint,
		printing.PermFindAllJobs,
		printing.PermFindJobByID,
	}
}

// operatorPermissions sind die Rechte der Bedienung: alles, was ein Gast
// darf, zuzüglich des Löschens und des Wiederholens von Druckaufträgen.
func operatorPermissions() []permission.ID {
	return append(guestPermissions(),
		photo.PermDelete,
		printing.PermRetry,
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
