// Command photoupld exposes the public upload relay for a private photobox.
// #[go.permission.generateTable]
package main

import (
	"log/slog"
	"time"

	"github.com/worldiety/option"
	"go.wdy.de/nago/application"
	"go.wdy.de/nago/pkg/std"
	"go.wdy.de/nago/web/vuejs"

	"github.com/torbenschinke/eventprint/app/photoupld/cfg"
	"github.com/torbenschinke/eventprint/pkg/heif"
)

func main() {
	// iPhones senden HEIC. Der Upload-Dienst braucht dafür libheif auf dem
	// Server; fehlt sie, bleibt es bei JPEG und PNG, und das Protokoll sagt es.
	if !heif.Register() {
		slog.Warn("photoupld: libheif nicht gefunden, HEIC-Fotos werden abgelehnt")
	}

	application.Configure(func(cfg *application.Configurator) {
		cfg.SetApplicationID("de.torbenschinke.photoupld")
		cfg.SetName("Fotobox Upload")
		cfg.SetSemanticVersion("0.1.0")
		cfg.SetHost("0.0.0.0")
		cfg.Serve(vuejs.Dist())
		option.MustZero(cfg.StandardSystems())
		users := std.Must(cfg.UserManagement())
		if std.Must(users.UseCases.CountUsers()) == 0 {
			std.Must(users.UseCases.EnableBootstrapAdmin(time.Now().Add(time.Hour), "%6UbRsCuM8N$auy"))
		}
		mgmt := std.Must(photoupld.Enable(cfg))
		cfg.SetDecorator(cfg.NewScaffold().Login(true).
			MenuEntry().Title("Meine Fotoboxen").Forward(mgmt.BoxesPage).OneOfRole(photoupld.OwnerRole).
			Decorator())
	}).Run()
}
