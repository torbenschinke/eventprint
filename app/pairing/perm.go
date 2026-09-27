package pairing

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

const (
	idRequestPairing permission.ID = "de.torbenschinke.photoupld.pairing.request"
	idConfirmPairing permission.ID = "de.torbenschinke.photoupld.pairing.confirm"
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
)

// Permissions nennt die Berechtigungen der Kopplung.
func Permissions() []permission.ID {
	return []permission.ID{PermRequestPairing, PermConfirmPairing}
}
