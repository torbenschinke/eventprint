package uidevice

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/app/lightroom"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/usb"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// lightroomData ist der Stand der Lightroom-Ansicht.
type lightroomData struct {
	account lightroom.AccountInfo
	albums  []lightroom.Album
}

// lightroomBrowser zeigt Alben und Fotos aus Adobe Lightroom.
//
// Gewählte Fotos werden erst beim Druck geholt, und zwar in der von
// Lightroom bearbeiteten Fassung. Was man in Lightroom entwickelt hat, kommt
// so auch aus dem Drucker.
func (a *App) lightroomBrowser(ctx *gift.Context, st *states) gift.View {
	album := ctx.State("album", "")
	selRev := ctx.State("lrSel", 0)
	busy := ctx.State("lrBusy", false)
	ctx.Read(selRev)

	info := xgift.UseResource[lightroomData](ctx, "lr")
	info.LoadOnce(func() (lightroomData, error) {
		c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		var d lightroomData
		var err error
		if d.account, err = a.dev.Lightroom.Account(a.dev.Subject(), c); err != nil || !d.account.Connected {
			return d, err
		}

		d.albums, err = a.dev.Lightroom.Albums(a.dev.Subject(), c)
		return d, err
	})

	d := info.Value()
	head := func(sub string) gift.View {
		return ui.HStack(
			ui.VStack(title("Adobe Lightroom", 30), muted(sub, 15)).Gap(u(2)).Flex(1),
		).PaddingInsets(geom.Insets{Top: u(14), Left: u(32), Right: u(32), Bottom: u(8)})
	}

	switch {
	case !info.Loaded():
		return xgift.Fill(ui.VStack(head("Verbindung wird geprüft …"), fill()).Background(ui.ColorSurface))
	case info.Err() != nil && !d.account.Connected:
		return ui.VStack(head(info.Err().Error()), fill()).Flex(1)
	case !d.account.Connected:
		return ui.VStack(
			head("Nicht verbunden"),
			ui.VStack(
				muted("Verbinde Lightroom einmal in den Einstellungen – die Anmeldung geschieht bequem auf dem Handy.", 17).MaxLines(3),
				primary("Zu den Einstellungen", func() {
					st.settings.Set(sectionSources)
					st.screen.Set(ScreenSettings)
				}),
			).Gap(u(16)).Padding(u(32)).Align(geom.TopLeading),
			fill(),
		).Flex(1)
	}

	chosen := album.Get()
	ctx.Read(album)

	albums := []gift.View{xgift.Chip("Alle Fotos", chosen == "", u(15), blue, func() { album.Set("") })}
	for _, al := range d.albums {
		albums = append(albums, xgift.Chip(al.Name, chosen == al.ID, u(15), blue, func() { album.Set(al.ID) }))
	}

	assets := xgift.UseResource[lightroom.Page](ctx, "assets")
	assets.LoadKeyed(chosen, func() (lightroom.Page, error) {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		return a.dev.Lightroom.Assets(a.dev.Subject(), c, chosen, "")
	})

	g := a.gallery("lightroom")
	page := assets.Value()
	if assets.Loaded() && a.refill("lightroom", lightroomKey(page.Assets)) {
		a.fillLightroom(g, page.Assets)
		xgift.ShowSelection(g, a.lrSelected)
	}

	more := gift.View(ui.Box().Frame(1, 1))
	if page.Next != "" {
		more = secondary("Mehr laden", func() {
			next := page.Next
			prev := page.Assets
			assets.Load(func() (lightroom.Page, error) {
				c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				p, err := a.dev.Lightroom.Assets(a.dev.Subject(), c, chosen, next)
				p.Assets = append(append([]lightroom.Asset{}, prev...), p.Assets...)
				return p, err
			})
		})
	}

	sub := "Verbunden"
	if d.account.Name != "" {
		sub = "Verbunden als " + d.account.Name
	}

	bar := gift.View(ui.Box().Frame(1, 1))
	if n := len(a.lrSelected); n > 0 {
		bar = importBar(n, "Gedruckt wird deine Lightroom-Bearbeitung", ctx.Read(busy), func() {
			ids := append([]asset.ID(nil), a.lrSelected...)
			busy.Set(true)
			a.importAsync(func(c context.Context) ([]photo.ID, error) {
				var out []photo.ID
				for _, id := range ids {
					dl, err := a.dev.Lightroom.Download(a.dev.Subject(), c, strings.TrimPrefix(string(id), "lr:"))
					if err != nil {
						return out, err
					}

					p, err := a.dev.Photos.Import(a.dev.Subject(), photo.ImportCmd{Name: dl.Name, Source: photo.SourceLightroom, Data: dl.Data})
					if err != nil {
						return out, err
					}

					out = append(out, p.ID)
				}

				return out, nil
			}, func() {
				busy.Set(false)
				a.lrSelected = nil
				selRev.Set(selRev.Get() + 1)
			})
		})
	}

	return ui.ZStack(
		xgift.Fill(ui.VStack(
			head(sub),
			ui.HScroll(albums...).Gap(u(8)).PaddingInsets(geom.Insets{Left: u(32), Right: u(32)}).MinHeight(u(48)),
			ui.ImageGallery(g).
				Layout(brickRows(u(170))).
				Tile(tileStyle()).
				PaddingInsets(geom.Insets{Left: u(32), Right: u(32), Top: u(12), Bottom: u(120)}).
				OnSelect(func(id asset.ID) {
					a.lrSelected = xgift.TouchSelect(g, a.lrSelected, id)
					selRev.Set(selRev.Get() + 1)
				}).
				Flex(1),
			more,
		).Gap(u(8)).Background(ui.ColorSurface)),
		bar,
	).Align(geom.Bottom).Flex(1)
}

// fillLightroom setzt die Kacheln einer Lightroom-Seite. Die Vorschaubilder
// kommen über den Anwendungsfall mit Anmeldung, nicht über eine offene URL.
func (a *App) fillLightroom(g *ui.Gallery, assets []lightroom.Asset) {
	meta := make([]asset.Metadata, 0, len(assets))
	for _, as := range assets {
		meta = append(meta, asset.Metadata{ID: asset.ID("lr:" + as.ID), Width: uint32(as.Width), Height: uint32(as.Height)})
	}

	g.SetCollection(asset.NewCollection(meta))
	g.SetSources(func(id asset.ID) asset.Source {
		assetID := strings.TrimPrefix(string(id), "lr:")
		return xgift.Func(string(id), assetID, func(c context.Context) (io.ReadCloser, error) {
			return a.dev.Lightroom.OpenThumbnail(a.dev.Subject(), c, assetID)
		})
	})
}

func lightroomKey(assets []lightroom.Asset) string {
	var b strings.Builder
	for _, a := range assets {
		b.WriteString(a.ID)
		b.WriteByte('|')
	}

	return b.String()
}

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

	action := "Übernehmen und drucken"
	if busy {
		action = "Wird übernommen …"
	}

	return floating(ui.HStack(
		ui.VStack(title(label, 17), muted(hint, 13)).Gap(u(2)).Flex(1),
		primary(action, fn).Disabled(busy),
	).Gap(u(12)).Align(geom.Center).
		PaddingInsets(geom.Insets{Left: u(22), Right: u(12), Top: u(10), Bottom: u(10)}).
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
		row := []gift.View{ui.VStack(title("USB-Stick", 30), muted(sub, 15)).Gap(u(2)).Flex(1)}
		return ui.HStack(append(row, extra...)...).Gap(u(12)).Align(geom.Center).
			PaddingInsets(geom.Insets{Top: u(14), Left: u(32), Right: u(32), Bottom: u(8)})
	}

	if len(d.drives) == 0 {
		hint := "Kein USB-Stick eingesteckt. Stecke einen Stick ein – er erscheint hier von selbst."
		if res.Err() != nil {
			hint = res.Err().Error()
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
				Layout(brickRows(u(170))).
				Tile(tileStyle()).
				PaddingInsets(geom.Insets{Left: u(32), Right: u(32), Bottom: u(120)}).
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
