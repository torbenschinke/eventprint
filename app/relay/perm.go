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

// Permissions liefert alle Berechtigungen dieses Kontexts.
func Permissions() []permission.ID { return []permission.ID{PermUploadAddress} }
