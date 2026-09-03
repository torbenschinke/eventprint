package wifi

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

// Je Anwendungsfall genau eine Berechtigung. Alle drei gehören der Betreuung:
// Ein Gast, der das Funknetz wechseln könnte, nähme der Fotobox mitten auf der
// Feier die Verbindung.
const (
	idScan    permission.ID = "de.torbenschinke.eventprint.wifi.scan"
	idStatus  permission.ID = "de.torbenschinke.eventprint.wifi.status"
	idConnect permission.ID = "de.torbenschinke.eventprint.wifi.connect"
)

var (
	PermScan = permission.Declare[Scan](idScan,
		permtext.Name(idScan, "Funknetze suchen", "Scan for wireless networks"),
		permtext.Description(idScan,
			"Träger dieser Berechtigung können nach verfügbaren Funknetzen suchen.",
			"Holders of this authorisation can search for available wireless networks."),
	)

	PermStatus = permission.Declare[Current](idStatus,
		permtext.Name(idStatus, "Funkverbindung anzeigen", "Show the wireless connection"),
		permtext.Description(idStatus,
			"Träger dieser Berechtigung können sehen, mit welchem Funknetz die Fotobox verbunden ist.",
			"Holders of this authorisation can see which wireless network the photo booth is connected to."),
	)

	PermConnect = permission.Declare[Connect](idConnect,
		permtext.Name(idConnect, "Mit einem Funknetz verbinden", "Connect to a wireless network"),
		permtext.Description(idConnect,
			"Träger dieser Berechtigung können die Fotobox mit einem Funknetz verbinden.",
			"Holders of this authorisation can connect the photo booth to a wireless network."),
	)
)
