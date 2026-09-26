package usb

import (
	"context"

	"go.wdy.de/nago/application/permission"
)

// Drives listet die USB-Sticks, die die Fotobox anbieten darf.
//
// Die Speicherkarte und jeder Datenträger, der das System trägt, fehlen
// absichtlich, auch wenn sie am USB hängen.
type Drives func(subject permission.Auditable, ctx context.Context) ([]Drive, error)

// NewDrives bindet die Auflistung an lsblk.
//
// lsblk statt udisksctl status: lsblk liefert JSON mit Transportweg und
// Einhängepunkt in einem Aufruf, und es braucht keine polkit-Freigabe.
func NewDrives(runner Runner) Drives {
	h := host{runner: runner}

	return func(subject permission.Auditable, ctx context.Context) ([]Drive, error) {
		if err := subject.Audit(PermDrives); err != nil {
			return nil, err
		}

		return h.drives(ctx)
	}
}
