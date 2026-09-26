package uidevice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/app/nas"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/usb"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

func usbKey(images []usb.Image) string {
	var b strings.Builder
	for _, img := range images {
		b.WriteString(img.Path)
		b.WriteByte('|')
	}

	return b.String()
}

// importBar ist die Leiste zum Übernehmen fremder Fotos.
func importBar(n int, hint string, busy bool, fn func()) gift.View {
	label := "1 Foto ausgewählt"
	if n > 1 {
		label = fmt.Sprintf("%d Fotos ausgewählt", n)
	}

	action := pick2("Übernehmen und drucken", "Übernehmen")
	if busy {
		action = "Wird übernommen …"
	}

	info := gift.View(ui.VStack(title(label, 17), muted(hint, 13).MaxLines(1)).Gap(u(2)).Flex(1))
	if compact {
		info = title(fmt.Sprintf("%d ausgewählt", n), 15).MaxLines(1).Flex(1)
	}

	return floating(ui.HStack(
		info,
		primary(action, fn).Disabled(busy),
	).Gap(u(12)).Align(geom.Center).
		PaddingInsets(geom.Insets{Left: u(pick(22, 14)), Right: u(pick(12, 8)), Top: u(pick(10, 6)), Bottom: u(pick(10, 6))}).
		Background(ui.ColorSurface).CornerRadius(u(22)).
		Shadow(ui.Shadow{Blur: u(30), OffsetY: u(10), Color: ui.RGBA(0, 0, 0, 60)}).
		Border(ui.Border{Width: 1, Color: ui.ColorSeparator}).
		MaxWidth(u(680)))
}

// importAsync holt Fotos im Hintergrund und öffnet danach das Druck-Studio.
func (a *App) importAsync(work func(context.Context) ([]photo.ID, error), done func()) {
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		ids, err := work(c)
		xgift.Post(func() {
			done()
			a.photosChanged()
			if err != nil {
				a.fail(err)
			}

			if len(ids) > 0 {
				a.startStudio(ids)
			}
		})
	}()
}

// usbData ist der Stand der USB-Ansicht.
type usbData struct {
	drives []usb.Drive
	images []usb.Image
}

// usbBrowser zeigt die Bilder auf einem eingesteckten USB-Stick.
func (a *App) usbBrowser(ctx *gift.Context, st *states) gift.View {
	tick := ctx.Read(st.tick)
	selRev := ctx.State("usbSel", 0)
	busy := ctx.State("usbBusy", false)
	ctx.Read(selRev)

	res := xgift.UseResource[usbData](ctx, "usb")
	load := func() (usbData, error) {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var d usbData
		var err error
		if d.drives, err = a.dev.USB.Drives(a.dev.Subject(), c); err != nil || len(d.drives) == 0 {
			return d, err
		}

		d.images, err = a.dev.USB.Images(a.dev.Subject(), c, d.drives[0].Path)
		return d, err
	}

	stamp := 0
	if !res.Loaded() || len(res.Value().drives) == 0 {
		// Solange kein Stick steckt, wird regelmäßig nachgesehen: Das
		// Einstecken soll genügen, ohne dass jemand irgendwo tippt.
		stamp = tick / 2
	}

	res.LoadKeyed(stamp, load)

	d := res.Value()
	header := func(sub string, extra ...gift.View) gift.View {
		row := []gift.View{ui.VStack(title("USB-Stick", pick(30, 22)), muted(sub, pick(15, 13))).Gap(u(2)).Flex(1)}
		return ui.HStack(append(row, extra...)...).Gap(u(12)).Align(geom.Center).
			PaddingInsets(geom.Insets{Top: u(pick(14, 8)), Left: u(pick(32, 16)), Right: u(pick(32, 16)), Bottom: u(8)})
	}

	if len(d.drives) == 0 {
		hint := "Kein USB-Stick eingesteckt. Stecke einen Stick ein – er erscheint hier von selbst."
		if res.Err() != nil {
			hint = humane(res.Err())
		}

		return ui.VStack(header(hint), fill()).Flex(1)
	}

	drive := d.drives[0]
	g := a.gallery("usb")
	if a.refill("usb", usbKey(d.images)) {
		meta := make([]asset.Metadata, 0, len(d.images))
		for _, img := range d.images {
			meta = append(meta, asset.Metadata{ID: asset.ID("usb:" + img.Path)})
		}

		g.SetCollection(asset.NewCollection(meta))
		g.SetSources(func(id asset.ID) asset.Source { return asset.File(strings.TrimPrefix(string(id), "usb:")) })
		xgift.ShowSelection(g, a.usbSelected)
	}

	bar := gift.View(ui.Box().Frame(1, 1))
	if n := len(a.usbSelected); n > 0 {
		bar = importBar(n, "Die Bilder werden auf die Box kopiert", ctx.Read(busy), func() {
			paths := append([]asset.ID(nil), a.usbSelected...)
			busy.Set(true)
			a.importAsync(func(c context.Context) ([]photo.ID, error) {
				var out []photo.ID
				for _, id := range paths {
					path := strings.TrimPrefix(string(id), "usb:")
					data, err := a.dev.USB.Read(a.dev.Subject(), c, path)
					if err != nil {
						return out, err
					}

					p, err := a.dev.Photos.Import(a.dev.Subject(), photo.ImportCmd{Name: filepath.Base(path), Source: photo.SourceUSB, Data: data})
					if err != nil {
						return out, err
					}

					out = append(out, p.ID)
				}

				return out, nil
			}, func() {
				busy.Set(false)
				a.usbSelected = nil
				selRev.Set(selRev.Get() + 1)
			})
		})
	}

	sub := fmt.Sprintf("%s · %s · %d Bilder", drive.Title(), drive.SizeText(), len(d.images))

	return ui.ZStack(
		xgift.Fill(ui.VStack(
			header(sub, secondary("Auswerfen", func() { a.eject(drive.Path, func() { res.Load(load) }) })),
			ui.ImageGallery(g).
				Layout(brickRows(u(rowHeight()))).
				Tile(tileStyle()).
				PaddingInsets(geom.Insets{Left: u(pick(32, 16)), Right: u(pick(32, 16)), Bottom: u(pick(120, 84))}).
				OnSelect(func(id asset.ID) {
					a.usbSelected = xgift.TouchSelect(g, a.usbSelected, id)
					selRev.Set(selRev.Get() + 1)
				}).
				Flex(1),
		).Background(ui.ColorSurface)),
		bar,
	).Align(geom.Bottom).Flex(1)
}

// eject wirft einen Stick aus und meldet, wann man ihn ziehen darf.
func (a *App) eject(path string, done func()) {
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := a.dev.USB.Eject(a.dev.Subject(), c, path)
		xgift.Post(func() {
			if a.fail(err) {
				return
			}

			done()
			a.show("Der USB-Stick kann jetzt abgezogen werden.")
		})
	}()
}

// nasBrowser zeigt die Ordner und Fotos der eingerichteten Netzwerkfreigabe.
//
// Ordner stehen als Chips über der Galerie, darüber der Pfad zum Zurückgehen.
// Die Auswahl gilt über Ordner hinweg: Wer im Urlaubsordner drei Bilder wählt
// und dann in den nächsten Ordner wechselt, verliert sie nicht.
func (a *App) nasBrowser(ctx *gift.Context, st *states) gift.View {
	dir := ctx.State("dir", ".")
	selRev := ctx.State("nasSel", 0)
	busy := ctx.State("nasBusy", false)
	retry := ctx.State("nasRetry", 0)
	ctx.Read(selRev)

	current := ctx.Read(dir)
	s, _ := a.dev.Device.LoadSettings(a.dev.Subject())
	cfg := s.NAS.Normalized()

	res := xgift.UseResource[nas.Listing](ctx, "nas")
	res.LoadKeyed([2]any{current, ctx.Read(retry)}, func() (nas.Listing, error) {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		return a.dev.NAS.Browse(a.dev.Subject(), c, current)
	})

	head := func(sub string, extra ...gift.View) gift.View {
		row := []gift.View{ui.VStack(title("NAS", pick(30, 22)), muted(sub, pick(15, 13)).MaxLines(2)).Gap(u(2)).Flex(1)}
		return ui.HStack(append(row, extra...)...).Gap(u(12)).Align(geom.Center).
			PaddingInsets(geom.Insets{Top: u(pick(14, 8)), Left: u(pick(32, 16)), Right: u(pick(32, 16)), Bottom: u(8)})
	}

	switch {
	case !cfg.Configured():
		return ui.VStack(
			head("Nicht eingerichtet"),
			ui.VStack(
				muted("Trage dein NAS einmal in den Einstellungen ein – Adresse, Benutzer und Kennwort. Danach erscheinen seine Fotos hier.", 17).MaxLines(3),
				primary("Zu den Einstellungen", func() {
					st.settings.Set(sectionSources)
					st.settingsOpen.Set(true)
					st.screen.Set(ScreenSettings)
				}),
			).Gap(u(16)).Padding(u(pick(32, 16))).Align(geom.TopLeading),
			fill(),
		).Flex(1)
	case !res.Loaded():
		return xgift.Fill(ui.VStack(head(cfg.Title()+" · wird geöffnet …"), fill()).Background(ui.ColorSurface))
	case res.Err() != nil:
		return ui.VStack(
			head(res.Err().Error(), secondary("Erneut versuchen", func() { retry.Set(retry.Get() + 1) })),
			fill(),
		).Flex(1)
	}

	l := res.Value()

	// Der Pfad: die Freigabe, dann jeder Ordner bis hierher.
	crumbs := []gift.View{xgift.Chip(cfg.Share, l.Dir == ".", u(15), blue, func() { dir.Set(".") })}
	if l.Dir != "." {
		parts := strings.Split(l.Dir, "/")
		for i, part := range parts {
			target := strings.Join(parts[:i+1], "/")
			crumbs = append(crumbs, muted("›", 17), xgift.Chip(part, i == len(parts)-1, u(15), blue, func() { dir.Set(target) }))
		}
	}

	var folders []gift.View
	for _, f := range l.Folders {
		folders = append(folders, xgift.Chip("▸ "+f.Name, false, u(15), blue, func() { dir.Set(f.Path) }))
	}

	g := a.gallery("nas")
	if a.refill("nas", cfg.Title()+"|"+nasKey(l.Images)) {
		a.fillNAS(g, cfg, l.Images)
		xgift.ShowSelection(g, a.nasSelected)
	}

	sub := fmt.Sprintf("%s · %d Ordner · %d Bilder", cfg.Title(), len(l.Folders), len(l.Images))
	if l.Truncated {
		sub += " (die ersten)"
	}

	bar := gift.View(ui.Box().Frame(1, 1))
	if n := len(a.nasSelected); n > 0 {
		bar = importBar(n, "Die Originale werden auf die Box kopiert", ctx.Read(busy), func() {
			ids := append([]asset.ID(nil), a.nasSelected...)
			busy.Set(true)
			a.importAsync(func(c context.Context) ([]photo.ID, error) {
				var out []photo.ID
				for _, id := range ids {
					p := strings.TrimPrefix(string(id), "nas:")
					data, err := a.dev.NAS.Read(a.dev.Subject(), c, p)
					if err != nil {
						return out, fmt.Errorf("%s: %w", path.Base(p), err)
					}

					ph, err := a.dev.Photos.Import(a.dev.Subject(), photo.ImportCmd{Name: path.Base(p), Source: photo.SourceNAS, Data: data})
					if err != nil {
						return out, err
					}

					out = append(out, ph.ID)
				}

				return out, nil
			}, func() {
				busy.Set(false)
				a.nasSelected = nil
				selRev.Set(selRev.Get() + 1)
			})
		})
	}

	rows := []gift.View{
		head(sub),
		ui.HScroll(crumbs...).Gap(u(6)).PaddingInsets(geom.Insets{Left: u(pick(32, 16)), Right: u(pick(32, 16))}).MinHeight(u(44)),
	}

	if len(folders) > 0 {
		rows = append(rows, ui.HScroll(folders...).Gap(u(8)).PaddingInsets(geom.Insets{Left: u(pick(32, 16)), Right: u(pick(32, 16))}).MinHeight(u(48)))
	}

	if len(l.Images) == 0 {
		hint := "In diesem Ordner liegen keine Fotos."
		if len(l.Folders) > 0 {
			hint = "Hier liegen nur Ordner. Tippe oben auf einen, um hineinzusehen."
		}

		rows = append(rows, muted(hint, 17).MaxLines(2).PaddingInsets(geom.Insets{Left: u(pick(32, 16)), Right: u(pick(32, 16)), Top: u(24)}), fill())
	} else {
		rows = append(rows, ui.ImageGallery(g).
			Layout(brickRows(u(rowHeight()))).
			Tile(tileStyle()).
			PaddingInsets(geom.Insets{Left: u(pick(32, 16)), Right: u(pick(32, 16)), Top: u(12), Bottom: u(pick(120, 84))}).
			OnSelect(func(id asset.ID) {
				a.nasSelected = xgift.TouchSelect(g, a.nasSelected, id)
				selRev.Set(selRev.Get() + 1)
			}).
			Flex(1))
	}

	return ui.ZStack(
		xgift.Fill(ui.VStack(rows...).Gap(u(8)).Background(ui.ColorSurface)),
		bar,
	).Align(geom.Bottom).Flex(1)
}

// fillNAS setzt die Kacheln eines Ordners. Die Vorschau kommt über den
// Anwendungsfall, der die fertigen Vorschaubilder des NAS nimmt; die Revision
// aus Größe und Änderungszeit sorgt dafür, dass ein ersetztes Foto nicht mit
// der alten Vorschau aus dem Zwischenspeicher erscheint.
func (a *App) fillNAS(g *ui.Gallery, cfg nas.Config, images []nas.Image) {
	meta := make([]asset.Metadata, 0, len(images))
	revs := make(map[asset.ID]string, len(images))
	for _, img := range images {
		id := asset.ID("nas:" + img.Path)
		meta = append(meta, asset.Metadata{ID: id})
		revs[id] = fmt.Sprintf("%s|%d|%d", cfg.Title(), img.Size, img.ModTime.Unix())
	}

	g.SetCollection(asset.NewCollection(meta))
	g.SetSources(func(id asset.ID) asset.Source {
		p := strings.TrimPrefix(string(id), "nas:")
		return xgift.Func(string(id), revs[id], func(c context.Context) (io.ReadCloser, error) {
			data, err := a.dev.NAS.Thumbnail(a.dev.Subject(), c, p)
			if err != nil {
				return nil, err
			}

			return io.NopCloser(bytes.NewReader(data)), nil
		})
	})
}

func nasKey(images []nas.Image) string {
	var b strings.Builder
	for _, img := range images {
		b.WriteString(img.Path)
		b.WriteByte('|')
	}

	return b.String()
}
