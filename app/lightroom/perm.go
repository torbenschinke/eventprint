package lightroom

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

// Je Anwendungsfall genau eine Berechtigung. Alle gehören dem Besitzer: Die
// Verbindung öffnet sein gesamtes Fotoarchiv, und ein Gast an der Box soll
// darin weder stöbern noch es mit einem eigenen Konto ersetzen können.
const (
	idBeginConnect  permission.ID = "de.torbenschinke.eventprint.lightroom.begin_connect"
	idAwaitConnect  permission.ID = "de.torbenschinke.eventprint.lightroom.await_connect"
	idDisconnect    permission.ID = "de.torbenschinke.eventprint.lightroom.disconnect"
	idAccount       permission.ID = "de.torbenschinke.eventprint.lightroom.account"
	idAlbums        permission.ID = "de.torbenschinke.eventprint.lightroom.albums"
	idAssets        permission.ID = "de.torbenschinke.eventprint.lightroom.assets"
	idOpenThumbnail permission.ID = "de.torbenschinke.eventprint.lightroom.open_thumbnail"
	idDownload      permission.ID = "de.torbenschinke.eventprint.lightroom.download"
)

var (
	PermBeginConnect = permission.Declare[BeginConnect](idBeginConnect,
		permtext.Name(idBeginConnect, "Lightroom-Verbindung beginnen", "Begin connecting Lightroom"),
		permtext.Description(idBeginConnect,
			"Träger dieser Berechtigung können einen QR-Code zur Anmeldung bei Adobe Lightroom anzeigen lassen.",
			"Holders of this authorisation can show a QR code for signing in to Adobe Lightroom."),
	)

	PermAwaitConnect = permission.Declare[AwaitConnect](idAwaitConnect,
		permtext.Name(idAwaitConnect, "Lightroom-Verbindung abschließen", "Complete the Lightroom connection"),
		permtext.Description(idAwaitConnect,
			"Träger dieser Berechtigung können eine auf dem Telefon abgeschlossene Anmeldung übernehmen.",
			"Holders of this authorisation can take over a sign-in completed on the phone."),
	)

	PermDisconnect = permission.Declare[Disconnect](idDisconnect,
		permtext.Name(idDisconnect, "Lightroom trennen", "Disconnect Lightroom"),
		permtext.Description(idDisconnect,
			"Träger dieser Berechtigung können die Verbindung zu Lightroom trennen.",
			"Holders of this authorisation can disconnect Lightroom."),
	)

	PermAccount = permission.Declare[Account](idAccount,
		permtext.Name(idAccount, "Lightroom-Verbindung anzeigen", "Show the Lightroom connection"),
		permtext.Description(idAccount,
			"Träger dieser Berechtigung können sehen, ob und mit welchem Konto Lightroom verbunden ist.",
			"Holders of this authorisation can see whether and with which account Lightroom is connected."),
	)

	PermAlbums = permission.Declare[Albums](idAlbums,
		permtext.Name(idAlbums, "Lightroom-Alben anzeigen", "Show Lightroom albums"),
		permtext.Description(idAlbums,
			"Träger dieser Berechtigung können die Alben des verbundenen Lightroom-Kontos sehen.",
			"Holders of this authorisation can see the albums of the connected Lightroom account."),
	)

	PermAssets = permission.Declare[Assets](idAssets,
		permtext.Name(idAssets, "Lightroom-Fotos auflisten", "List Lightroom photos"),
		permtext.Description(idAssets,
			"Träger dieser Berechtigung können die Fotos eines Lightroom-Albums auflisten.",
			"Holders of this authorisation can list the photos of a Lightroom album."),
	)

	PermOpenThumbnail = permission.Declare[OpenThumbnail](idOpenThumbnail,
		permtext.Name(idOpenThumbnail, "Lightroom-Vorschau laden", "Load a Lightroom preview"),
		permtext.Description(idOpenThumbnail,
			"Träger dieser Berechtigung können Vorschaubilder aus Lightroom laden.",
			"Holders of this authorisation can load preview images from Lightroom."),
	)

	PermDownload = permission.Declare[Download](idDownload,
		permtext.Name(idDownload, "Lightroom-Foto herunterladen", "Download a Lightroom photo"),
		permtext.Description(idDownload,
			"Träger dieser Berechtigung können Fotos in Druckqualität aus Lightroom auf die Fotobox laden.",
			"Holders of this authorisation can download photos in print quality from Lightroom to the photo booth."),
	)
)

// Permissions sind alle Berechtigungen dieses Pakets, für die Rolle des
// Besitzers.
func Permissions() []permission.ID {
	return []permission.ID{
		PermBeginConnect,
		PermAwaitConnect,
		PermDisconnect,
		PermAccount,
		PermAlbums,
		PermAssets,
		PermOpenThumbnail,
		PermDownload,
	}
}
