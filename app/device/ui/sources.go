package uidevice

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"

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
