package relay

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

const idUploadAddress permission.ID = "de.torbenschinke.eventprint.relay.upload_address"

// PermUploadAddress erlaubt, den QR-Code zum Hochladen zu zeigen. Auch Gäste
// haben sie – der Code ist die Einladung an sie.
var PermUploadAddress = permission.Declare[UploadAddress](idUploadAddress,
	permtext.Name(idUploadAddress, "QR-Code zum Hochladen zeigen", "Show the upload QR code"),
	permtext.Description(idUploadAddress,
		"Träger dieser Berechtigung sehen die Adresse, unter der Handys Bilder an das Gerät senden.",
		"Holders of this authorisation see the address phones use to send photos to the device."),
)

const idPair permission.ID = "de.torbenschinke.eventprint.relay.pair"

// PermPair erlaubt, die Box mit einem Konto des Upload-Dienstes zu koppeln.
// Nur der Besitzer hat sie; ein Gast soll die Box nicht an sein eigenes Konto
// hängen können.
var PermPair = permission.Declare[BeginPairing](idPair,
	permtext.Name(idPair, "Mit dem Upload-Dienst koppeln", "Pair with the upload service"),
	permtext.Description(idPair,
		"Träger dieser Berechtigung können die Fotobox mit einem Konto des Upload-Dienstes verbinden.",
		"Holders of this authorisation can connect the photo booth to an upload service account."),
)

const idCompletePair permission.ID = "de.torbenschinke.eventprint.relay.complete_pair"

// PermCompletePair erlaubt, eine Kopplung mit dem Code abzuschließen.
var PermCompletePair = permission.Declare[CompletePairing](idCompletePair,
	permtext.Name(idCompletePair, "Kopplung abschließen", "Complete the pairing"),
	permtext.Description(idCompletePair,
		"Träger dieser Berechtigung können den Code eintippen, der die Fotobox mit dem Upload-Dienst verbindet.",
		"Holders of this authorisation can enter the code that connects the photo booth to the upload service."),
)

// Permissions liefert alle Berechtigungen dieses Kontexts.
func Permissions() []permission.ID {
	return []permission.ID{PermUploadAddress, PermPair, PermCompletePair}
}
