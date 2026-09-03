// Package uiphotobox enthält die Oberfläche der Fotobox.
//
// Die Seiten sind bewusst für zwei sehr unterschiedliche Geräte ausgelegt:
// den großen Touchscreen der Fotobox (Startbildschirm, Galerie, Druckstatus)
// und das Smartphone eines Gastes (Upload-Seite, per QR-Code erreichbar).
package uiphotobox

import (
	"go.wdy.de/nago/presentation/core"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

// Pages bündelt die Navigationspfade aller Seiten der Fotobox.
type Pages struct {
	// Booth ist der Startbildschirm auf dem Fotobox-Display.
	Booth core.NavigationPath

	// Upload ist die öffentliche Seite, die Gäste per QR-Code aufrufen.
	Upload core.NavigationPath

	// Gallery zeigt die vollständige Historie aller Fotos.
	Gallery core.NavigationPath

	// Jobs zeigt den Status aller Druckaufträge.
	Jobs core.NavigationPath

	// WiFi ist die Einrichtung der Funkverbindung.
	WiFi core.NavigationPath
}

// Options bündelt alles, was die Seiten zur Arbeit brauchen.
type Options struct {
	Pages    Pages
	Photos   photo.UseCases
	Printing printing.UseCases

	// EventTitle liefert die Überschrift des Startbildschirms,
	// z. B. "Hochzeit von Anna & Ben". Sie wird bei jedem Aufruf gelesen,
	// damit eine Änderung in den Einstellungen sofort sichtbar ist.
	EventTitle func() string

	// PrinterSettings verweist auf die Nago-Einstellungsseite, auf der der
	// Drucker gewählt wird. Sie liegt im Admin-Center und ist deshalb nur für
	// angemeldete Betreuer erreichbar.
	PrinterSettings core.NavigationPath

	// PrinterSettingsParams wählt auf der Einstellungsseite den richtigen
	// Abschnitt aus.
	PrinterSettingsParams core.Values

	// BoothSettings verweist auf die Einstellungen der Fotobox selbst, also
	// Veranstaltungstitel und öffentliche Adresse.
	BoothSettings core.NavigationPath

	// BoothSettingsParams wählt dort den richtigen Abschnitt aus.
	BoothSettingsParams core.Values

	// ArchiveDir ist der Ordner, in dem jedes eingehende Bild zusätzlich im
	// Original liegt. Er wird nur angezeigt, damit die Betreuung ihn nach der
	// Feier findet, ohne die Anwendung zu befragen.
	ArchiveDir string

	// Pin ist der Zugang zur Einrichtung ueber das Tastenfeld.
	Pin PinAccess

	// CanConfigure meldet, ob die aktuelle Sitzung einrichten darf.
	//
	// Frueher stand hier ueberall Subject().Valid(), also "ist angemeldet?".
	// Das ist der falsche Massstab: Gefragt ist, ob jemand einrichten DARF.
	// Die Verwechslung sperrte jede Anmeldeart aus, die ohne Benutzerkonto
	// auskommt - und genau so eine ist die PIN.
	CanConfigure func(wnd core.Window) bool

	// UploadURL liefert die absolute, von außen erreichbare Adresse der
	// Upload-Seite. Sie wird erst zur Laufzeit gebildet, weil Nago den
	// öffentlichen Hostnamen aus der ersten Verbindung ableitet.
	UploadURL func() string
}
