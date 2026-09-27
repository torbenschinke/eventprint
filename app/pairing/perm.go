package pairing

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

const (
	idRequestPairing permission.ID = "de.torbenschinke.photoupld.pairing.request"
	idConfirmPairing permission.ID = "de.torbenschinke.photoupld.pairing.confirm"
	idFindMyBoxes    permission.ID = "de.torbenschinke.photoupld.pairing.find_my_boxes"
	idUnpairBox      permission.ID = "de.torbenschinke.photoupld.pairing.unpair_box"
)

var (
	PermRequestPairing = permission.Declare[RequestPairing](idRequestPairing,
		permtext.Name(idRequestPairing, "Kopplungscode anfordern", "Request a pairing code"),
		permtext.Description(idRequestPairing,
			"Träger dieser Berechtigung können für eine Fotobox einen Code an ein Konto schicken lassen.",
			"Holders of this authorisation can have a pairing code sent to an account for a photo booth."),
	)

	PermConfirmPairing = permission.Declare[ConfirmPairing](idConfirmPairing,
		permtext.Name(idConfirmPairing, "Kopplung bestätigen", "Confirm a pairing"),
		permtext.Description(idConfirmPairing,
			"Träger dieser Berechtigung können einen Kopplungscode einlösen und so ein Zugangstoken für eine Fotobox erhalten.",
			"Holders of this authorisation can redeem a pairing code and receive an access token for a photo booth."),
	)

	PermFindMyBoxes = permission.Declare[FindMyBoxes](idFindMyBoxes,
		permtext.Name(idFindMyBoxes, "Eigene Fotoboxen anzeigen", "Show own photo booths"),
		permtext.Description(idFindMyBoxes,
			"Träger dieser Berechtigung sehen die Fotoboxen, die sie mit ihrem Konto gekoppelt haben.",
			"Holders of this authorisation see the photo booths they paired with their account."),
	)

	PermUnpairBox = permission.Declare[UnpairBox](idUnpairBox,
		permtext.Name(idUnpairBox, "Eigene Fotobox trennen", "Unpair own photo booth"),
		permtext.Description(idUnpairBox,
			"Träger dieser Berechtigung können eine eigene Fotobox trennen; sie holt danach keine Uploads mehr ab.",
			"Holders of this authorisation can unpair one of their photo booths; it no longer fetches uploads."),
	)
)

// Permissions nennt die Berechtigungen der Kopplung.
func Permissions() []permission.ID {
	return []permission.ID{PermRequestPairing, PermConfirmPairing, PermFindMyBoxes, PermUnpairBox}
}

// OwnerPermissions sind die Rechte der Rolle Fotobox-Besitzer.
func OwnerPermissions() []permission.ID {
	return []permission.ID{PermFindMyBoxes, PermUnpairBox}
}
