// Command photoupld exposes the public upload relay for a private photobox.
// #[go.permission.generateTable]
package main

import (
	"time"

	"github.com/worldiety/option"
	"go.wdy.de/nago/application"
	"go.wdy.de/nago/pkg/std"
	"go.wdy.de/nago/web/vuejs"

	"github.com/torbenschinke/eventprint/photoupld"
)

func main() {
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
		std.Must(photoupld.Enable(cfg))
		cfg.SetDecorator(cfg.NewScaffold().Login(true).Decorator())
	}).Run()
}
