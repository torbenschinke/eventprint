package usb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"go.wdy.de/nago/application/permission"
)

// Eject hängt einen Stick aus und schaltet ihn ab, damit er gefahrlos
// abgezogen werden kann.
type Eject func(subject permission.Auditable, ctx context.Context, drive string) error

// BusyError meldet, dass der Stick noch in Benutzung ist.
//
// Ein eigener Typ, weil die Bedienung eine andere ist als bei jedem anderen
// Fehler: nicht abziehen, kurz warten, noch einmal versuchen.
type BusyError struct {
	Path string
}

func (e BusyError) Error() string {
	return "der USB-Stick wird gerade noch benutzt; bitte einen Moment warten und dann erneut auswerfen, erst danach abziehen"
}

// NewEject bindet das Auswerfen an udisksctl.
//
// Aushängen allein genügt nicht: Viele Sticks halten Daten im eigenen Cache,
// bis sie abgeschaltet werden. Erst power-off gibt die Gewissheit, die ein
// Gastgeber braucht, der den Stick danach sofort einsteckt und heimfährt.
func NewEject(runner Runner) Eject {
	h := host{runner: runner}

	return func(subject permission.Auditable, ctx context.Context, drive string) error {
		if err := subject.Audit(PermEject); err != nil {
			return err
		}

		d, err := h.find(ctx, drive)
		if err != nil {
			// Ein Stick, der nicht mehr steckt, ist ausgeworfen. Ob er
			// sauber abgezogen wurde, lässt sich jetzt ohnehin nicht mehr
			// ändern.
			var gone DriveNotFoundError
			if errors.As(err, &gone) {
				return nil
			}

			return err
		}

		if d.MountPoint != "" {
			out, err := run(ctx, h.runner, unmountTimeout, "udisksctl", "unmount", "--no-user-interaction", "-b", d.Path)
			if err != nil {
				msg := string(out) + err.Error()

				switch {
				case strings.Contains(msg, "NotMounted"), strings.Contains(msg, "is not mounted"):
					// Zwischen lsblk und udisksctl ausgehängt: Ziel erreicht.
				case strings.Contains(strings.ToLower(msg), "busy"):
					// "target is busy" vom Kernel, DeviceBusy von udisks: Ein
					// Programm hat noch eine Datei offen, meist ein laufender
					// Export oder eine Vorschau.
					return BusyError{Path: d.Path}
				default:
					return fmt.Errorf("der USB-Stick %s lässt sich nicht aushängen: %w", d.Title(), err)
				}
			}
		}

		// Das Abschalten ist eine Zugabe. Scheitert es, etwa weil noch eine
		// zweite Partition eingehängt ist oder der Kartenleser es nicht
		// kann, ist der Stick trotzdem ausgehängt und alle Daten sind
		// geschrieben. Den Gastgeber deshalb warten zu lassen, hilft ihm
		// nicht.
		if _, err := run(ctx, h.runner, powerOffTimeout, "udisksctl", "power-off", "--no-user-interaction", "-b", d.Disk); err != nil {
			slog.Warn("usb drive unmounted but not powered off", "drive", d.Path, "disk", d.Disk, "err", err)
		}

		return nil
	}
}
