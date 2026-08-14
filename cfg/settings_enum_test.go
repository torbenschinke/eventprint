package cfgphotobox_test

import (
	"reflect"
	"testing"

	"github.com/worldiety/enum"
	"go.wdy.de/nago/application/settings"
)

// TestSettingsVariantsHaveDistinctNames schützt vor einem Fehler, der zur
// Laufzeit nur als schwer deutbarer Typkonflikt auffällt:
//
//	interface conversion: settings.GlobalSettings is cfgphotobox.Settings,
//	not printing.Settings
//
// Ursache ist, dass enum standardmäßig den bloßen Go-Typnamen als
// Diskriminator im JSON verwendet. Mehrere Pakete nennen ihren
// Einstellungstyp "Settings" – ohne enum.Rename überschreiben sie sich
// gegenseitig, und gespeicherte Einstellungen werden als falscher Typ
// ausgepackt.
//
// Der Test prüft deshalb alle registrierten Varianten, nicht nur die eigenen:
// Ein späterer Einstellungstyp fällt damit sofort auf.
func TestSettingsVariantsHaveDistinctNames(t *testing.T) {
	decl, ok := enum.DeclarationFor[settings.GlobalSettings]()
	if !ok {
		t.Fatal("es ist keine einzige Einstellungsvariante registriert")
	}

	byName := map[string]reflect.Type{}
	var ours int

	for variant := range decl.Variants() {
		name, ok := decl.Name(variant)
		if !ok {
			t.Errorf("Variante %v hat keinen Namen", variant)
			continue
		}

		if name == variant.Name() {
			t.Errorf("Variante %v nutzt den bloßen Typnamen %q als Diskriminator; enum.Rename fehlt", variant, name)
		}

		if other, exists := byName[name]; exists {
			t.Errorf("Diskriminator %q wird von %v und %v gleichzeitig verwendet", name, other, variant)
		}

		byName[name] = variant

		if variant.PkgPath() == "github.com/torbenschinke/eventprint/cfg" ||
			variant.PkgPath() == "github.com/torbenschinke/eventprint/printing" {
			ours++
		}
	}

	// Ohne diese Prüfung liefe der Test auch grün, wenn die Registrierung der
	// eigenen Typen versehentlich entfällt.
	if ours != 2 {
		t.Errorf("es sind %d eigene Einstellungstypen registriert, erwartet 2", ours)
	}
}
