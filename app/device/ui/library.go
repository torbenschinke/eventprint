package uidevice

import (
	"fmt"
	"slices"
	"strings"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/icon/outline"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/app/device"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// Quellen außerhalb der Mediathek.
const (
	sourceNAS = "nas"
	sourceUSB = "usb"
)

// openLibrary öffnet die Mediathek mit einer Auswahl.
func (a *App) openLibrary(scope photo.Scope, source string) {
	a.clearSelection()
	a.st.libScope.Set(scope)
	a.st.libSource.Set(source)
	a.st.screen.Set(ScreenLibrary)
}

func (a *App) clearSelection() {
	a.selected = nil
	a.st.selection.Set(a.st.selection.Get() + 1)
}

// library ist die Mediathek: links die Seitenleiste, rechts die Fotos.
func (a *App) library(ctx *gift.Context, st *states) gift.View {
	source := ctx.Read(st.libSource)

	var content gift.View
	switch source {
	case sourceNAS:
		content = gift.Component("nas", func(ctx *gift.Context) gift.View { return a.nasBrowser(ctx, st) })
	case sourceUSB:
		content = gift.Component("usb", func(ctx *gift.Context) gift.View { return a.usbBrowser(ctx, st) })
	default:
		content = gift.Component("photos", func(ctx *gift.Context) gift.View { return a.photoBrowser(ctx, st) })
	}

	return xgift.HStretch(
		gift.Component("sidebar", func(ctx *gift.Context) gift.View { return a.librarySidebar(ctx, st) }),
		grow(content),
	).Flex(1)
}

// sidebarData sind die Zahlen der Seitenleiste.
type sidebarData struct {
	counts map[photo.Scope]int
	events []device.Event
}

func (a *App) librarySidebar(ctx *gift.Context, st *states) gift.View {
	scope := ctx.Read(st.libScope)
	event := ctx.Read(st.libEvent)
	source := ctx.Read(st.libSource)
	rev := ctx.Read(st.photos)

	res := xgift.UseResource[sidebarData](ctx, "sidebar")
	res.LoadKeyed(rev, func() (sidebarData, error) {
		d := sidebarData{counts: map[photo.Scope]int{}}
		subject := a.dev.Subject()
		for _, sc := range []photo.Scope{photo.ScopeInbox, photo.ScopeAll, photo.ScopeFavorites, photo.ScopePrinted} {
			list, err := a.dev.Photos.FindAll(subject, photo.Query{Scope: sc})
			if err != nil {
				return d, err
			}

			d.counts[sc] = len(list)
		}

		s, err := a.dev.Device.LoadSettings(subject)
		if err != nil {
			return d, err
		}

		d.events = slices.Clone(s.Events)
		slices.Reverse(d.events)
		if len(d.events) > 5 {
			d.events = d.events[:5]
		}

		return d, nil
	})

	d := res.Value()

	row := func(label string, sym ui.Symbol, count string, active bool, fn func()) gift.View {
		fg, ic, face := ui.ColorLabel, ui.ColorAccent, ui.ColorClear
		if active {
			fg, ic, face = white, white, blue
		}

		style := ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: u(10)}

		return ui.Button(ui.HStack(
			ui.Icon(sym).Size(u(pick(22, 18))).Foreground(ic),
			ui.Text(label).FontSize(u(pick(16, 14))).Foreground(fg).MaxLines(1).Flex(1),
			ui.Text(count).FontSize(u(pick(14, 12))).Foreground(fg),
		).Gap(u(pick(12, 8))).Align(geom.Center), fn).
			Style(style).HoverStyle(style).
			PressedStyle(ui.ButtonStyle{Background: ui.Fade(blue, 0.2), CornerRadius: u(10)}).
			PaddingInsets(geom.Insets{Left: u(pick(12, 8)), Right: u(pick(12, 8))}).
			MinHeight(u(pick(44, 40)))
	}

	count := func(sc photo.Scope) string {
		if n, ok := d.counts[sc]; ok && n > 0 {
			return fmt.Sprint(n)
		}

		return ""
	}

	local := source == ""
	items := []gift.View{
		ui.HStack(
			iconButton(outline.Home, "Home", ui.ColorAccent, func() { st.screen.Set(ScreenHome) }),
			fill(),
			iconButton(outline.List, "Aufträge", ui.ColorAccent, func() { st.screen.Set(ScreenJobs) }),
		).Align(geom.Center),
	}

	if compact {
		// Auf kleinen Panels steht der Titel zwischen den Knöpfen, statt eine
		// eigene Zeile zu kosten.
		items[0] = ui.HStack(
			iconButton(outline.Home, "Home", ui.ColorAccent, func() { st.screen.Set(ScreenHome) }),
			title("Fotos", 20).Flex(1).PaddingInsets(geom.Insets{Left: u(6)}),
			iconButton(outline.List, "Aufträge", ui.ColorAccent, func() { st.screen.Set(ScreenJobs) }),
		).Align(geom.Center)
	} else {
		items = append(items, title("Fotos", 30).PaddingInsets(geom.Insets{Left: u(8), Bottom: u(6)}))
	}

	items = append(items,
		muted("MEDIATHEK", 13).PaddingInsets(geom.Insets{Left: u(12), Top: u(4)}),
		row("Eingang", outline.Inbox, count(photo.ScopeInbox), local && scope == photo.ScopeInbox, func() { a.openLibrary(photo.ScopeInbox, "") }),
		row("Alle Fotos", outline.Grid, count(photo.ScopeAll), local && scope == photo.ScopeAll, func() { a.openLibrary(photo.ScopeAll, "") }),
		row("Favoriten", outline.Heart, count(photo.ScopeFavorites), local && scope == photo.ScopeFavorites, func() { a.openLibrary(photo.ScopeFavorites, "") }),
		row("Gedruckt", outline.Printer, count(photo.ScopePrinted), local && scope == photo.ScopePrinted, func() { a.openLibrary(photo.ScopePrinted, "") }),
	)

	if len(d.events) > 0 {
		items = append(items, muted("FEIERN", 13).PaddingInsets(geom.Insets{Left: u(12), Top: u(pick(14, 8))}))
		for _, e := range d.events {
			items = append(items, row(e.Title, outline.WandMagicSparkles, e.StartedAt.Local().Format("02.01."),
				local && scope == photo.ScopeEvent && event == e.ID, func() {
					a.st.libEvent.Set(e.ID)
					a.openLibrary(photo.ScopeEvent, "")
				}))
		}
	}

	items = append(items,
		muted("QUELLEN", 13).PaddingInsets(geom.Insets{Left: u(12), Top: u(pick(14, 8))}),
		row("NAS", outline.Server, "", source == sourceNAS, func() { a.openLibrary(scope, sourceNAS) }),
		row("USB-Stick", outline.ArchiveArrowDown, "", source == sourceUSB, func() { a.openLibrary(scope, sourceUSB) }),
	)

	return xgift.Fill(ui.VScroll(ui.VStack(items...).Gap(u(pick(4, 2))).Padding(u(pick(16, 10)))).
		Background(ui.ColorBackground)).Width(u(pick(300, clamp(vw()*0.27, 190, 250))))
}

// gallery liefert die dauerhafte Galerie eines Bildschirms.
func (a *App) gallery(key string) *ui.Gallery {
	g, ok := a.galleries[key]
	if !ok {
		g = ui.NewGallery(asset.NewCollection(nil))
		a.galleries[key] = g
	}

	return g
}

// refill meldet, ob eine Galerie einen neuen Inhalt braucht, und merkt sich
// den Inhalt, den sie bekommt.
//
// Verglichen wird der Inhalt selbst und nicht, ob neu geladen wurde. Der
// Kiosk und der Eingang laden im Takt nach; eine Galerie, die dabei jedes Mal
// neu befüllt würde, bände alle Kacheln neu und ließe die Bilder flackern,
// obwohl sich nichts geändert hat.
func (a *App) refill(key string, version any) bool {
	if a.filled == nil {
		a.filled = map[string]any{}
	}

	if v, ok := a.filled[key]; ok && v == version {
		return false
	}

	a.filled[key] = version

	return true
}

// tileStyle ist das Aussehen einer Kachel.
func tileStyle() ui.TileStyle {
	return ui.TileStyle{
		// Foto, Platzhalter und Auswahlrahmen teilen sich die Rundung.
		CornerRadius: u(6),
		Palette:      []ui.Color{tileA, tileB},
		Error:        tileError,
		Selected:     ui.Border{Width: u(5), Color: blue},
	}
}

// photoList ist eine geladene Liste von Fotos.
type photoList struct {
	items []photo.Location
}

// fillGallery setzt den Inhalt einer Galerie aus Originaldateien.
func fillGallery(g *ui.Gallery, items []photo.Location) {
	meta := make([]asset.Metadata, 0, len(items))
	paths := make(map[asset.ID]string, len(items))
	for _, it := range items {
		id := asset.ID(it.Photo.ID)
		meta = append(meta, asset.Metadata{ID: id, Width: uint32(it.Photo.Width), Height: uint32(it.Photo.Height)})
		paths[id] = it.Path
	}

	g.SetCollection(asset.NewCollection(meta))
	g.SetSources(func(id asset.ID) asset.Source {
		if p, ok := paths[id]; ok {
			return asset.File(p)
		}

		return nil
	})
}

// rowHeight ist die Höhe einer Ziegelreihe: auf großen Bildschirmen 190
// Punkte, auf kleinen so, dass gut zwei Reihen zu sehen sind.
func rowHeight() float32 {
	return pick(190, clamp(vh()*0.3, 110, 170))
}

// brickRows legt die Fotos zeilenweise wie Ziegel: gleich hohe Reihen, jedes
// Bild in seinem eigenen Seitenverhältnis. Ein quadratischer Ausschnitt
// zeigte nicht, was tatsächlich auf dem Foto ist – und genau das will man vor
// dem Druck sehen.
func brickRows(rowHeight float32) ui.GalleryLayout {
	return ui.Justified().RowHeight(rowHeight).Gap(u(6))
}

// locationKey fasst den Inhalt einer Liste für den Vergleich in refill.
func locationKey(items []photo.Location) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString(string(it.Photo.ID))
		b.WriteByte('|')
	}

	return b.String()
}

func scopeTitle(scope photo.Scope) (string, string) {
	switch scope {
	case photo.ScopeInbox:
		return "Eingang", "Vom Handy und von der Kamera"
	case photo.ScopeFavorites:
		return "Favoriten", "Von dir markiert"
	case photo.ScopePrinted:
		return "Gedruckt", "Schon einmal auf Papier"
	case photo.ScopeEvent:
		return "Feier", "Fotos einer Feier im Kiosk"
	default:
		return "Alle Fotos", "Alles auf der Box"
	}
}

// photoBrowser zeigt die Fotos der Mediathek.
func (a *App) photoBrowser(ctx *gift.Context, st *states) gift.View {
	scope := ctx.Read(st.libScope)
	event := ctx.Read(st.libEvent)
	rev := ctx.Read(st.photos)
	tick := ctx.Read(st.tick)
	ctx.Read(st.selection)

	// Der Eingang füllt sich von selbst, während man davorsteht. Die anderen
	// Listen ändern sich nur durch eigene Aktionen.
	refresh := 0
	if scope == photo.ScopeInbox {
		refresh = tick / 3
	}

	res := xgift.UseResource[photoList](ctx, "list")
	res.LoadKeyed([4]any{scope, event, rev, refresh}, func() (photoList, error) {
		subject := a.dev.Subject()
		list, err := a.dev.Photos.FindAll(subject, photo.Query{Scope: scope, Event: event})
		if err != nil {
			return photoList{}, err
		}

		ids := make([]photo.ID, 0, len(list))
		for _, p := range list {
			ids = append(ids, p.ID)
		}

		locs, err := a.dev.Photos.Locate(subject, ids...)
		return photoList{items: locs}, err
	})

	g := a.gallery("photos")
	list := res.Value()
	if res.Loaded() && a.refill("photos", locationKey(list.items)) {
		fillGallery(g, list.items)
		xgift.ShowSelection(g, toAssetIDs(a.selected))
	}

	name, hint := scopeTitle(scope)
	if res.Err() != nil {
		hint = res.Err().Error()
	}

	header := ui.HStack(
		ui.VStack(
			title(name, pick(30, 22)),
			muted(fmt.Sprintf("%s · %d Fotos", hint, len(list.items)), 15),
		).Gap(u(2)).Flex(1),
		a.selectAllButton(list.items),
	).Gap(u(12)).Align(geom.Center).PaddingInsets(geom.Insets{Top: u(pick(14, 8)), Left: u(pick(32, 16)), Right: u(pick(32, 16)), Bottom: u(8)})

	var grid gift.View
	if res.Loaded() && len(list.items) == 0 {
		grid = ui.VStack(muted(emptyHint(scope), 17).MaxLines(3)).Align(geom.Center).Flex(1).Padding(u(40))
	} else {
		grid = ui.ImageGallery(g).
			Layout(brickRows(u(rowHeight()))).
			Tile(tileStyle()).
			Overscan(u(400)).
			PaddingInsets(geom.Insets{Left: u(pick(32, 16)), Right: u(pick(32, 16)), Bottom: u(pick(120, 84))}).
			OnSelect(func(id asset.ID) {
				a.selected = toPhotoIDs(xgift.TouchSelect(g, toAssetIDs(a.selected), id))
				st.selection.Set(st.selection.Get() + 1)
			}).
			Flex(1)
	}

	return ui.ZStack(
		xgift.Fill(ui.VStack(header, grid).Background(ui.ColorSurface)),
		a.selectionBar(st),
	).Align(geom.Bottom)
}

func emptyHint(scope photo.Scope) string {
	switch scope {
	case photo.ScopeInbox:
		return "Der Eingang ist leer. Scanne den QR-Code auf dem Home-Bildschirm mit dem Handy und sende Fotos."
	case photo.ScopeFavorites:
		return "Noch keine Favoriten. Wähle Fotos aus und tippe auf das Herz."
	case photo.ScopePrinted:
		return "Noch nichts gedruckt."
	default:
		return "Noch keine Fotos auf der Box."
	}
}

func (a *App) selectAllButton(items []photo.Location) gift.View {
	if len(a.selected) > 0 {
		return secondary("Auswahl aufheben", func() {
			a.clearSelection()
			xgift.ShowSelection(a.gallery("photos"), nil)
		})
	}

	if len(items) == 0 {
		return ui.Box().Frame(1, 1)
	}

	return secondary("Alle auswählen", func() {
		a.selected = nil
		for _, it := range items {
			a.selected = append(a.selected, it.Photo.ID)
		}

		xgift.ShowSelection(a.gallery("photos"), toAssetIDs(a.selected))
		a.st.selection.Set(a.st.selection.Get() + 1)
	})
}

// selectionBar schwebt über der Galerie, sobald etwas ausgewählt ist.
func (a *App) selectionBar(st *states) gift.View {
	n := len(a.selected)
	if n == 0 {
		return ui.Box().Frame(1, 1)
	}

	label := "1 Foto ausgewählt"
	if n > 1 {
		label = fmt.Sprintf("%d Fotos ausgewählt", n)
	}

	if compact {
		label = fmt.Sprintf("%d ausgewählt", n)
	}

	return floating(ui.HStack(
		title(label, pick(17, 15)).MaxLines(1).Flex(1),
		iconButton(outline.Heart, "Favorit", ui.ColorAccent, func() {
			if !a.fail(a.dev.Photos.SetFavorite(a.dev.Subject(), true, a.selected...)) {
				a.show("Als Favorit markiert.")
				a.photosChanged()
			}
		}),
		iconButton(outline.ArchiveArrowDown, "Auf USB-Stick kopieren", ui.ColorAccent, func() { a.openSheet(SheetExport) }),
		iconButton(outline.TrashBin, "Löschen", red, func() { a.openSheet(SheetConfirmDelete) }),
		primary(pick2("Weiter zum Drucken", "Drucken"), func() { a.startStudio(slices.Clone(a.selected)) }),
	).Gap(u(pick(12, 8))).Align(geom.Center).
		PaddingInsets(geom.Insets{Left: u(pick(22, 14)), Right: u(pick(12, 8)), Top: u(pick(10, 6)), Bottom: u(pick(10, 6))}).
		Background(ui.ColorSurface).CornerRadius(u(22)).
		Shadow(ui.Shadow{Blur: u(30), OffsetY: u(10), Color: ui.RGBA(0, 0, 0, 60)}).
		Border(ui.Border{Width: 1, Color: ui.ColorSeparator}).
		MaxWidth(u(760)))
}

// startStudio öffnet das Druck-Studio mit den Fotos.
func (a *App) startStudio(ids []photo.ID) {
	if len(ids) == 0 {
		return
	}

	a.studio = ids
	a.st.active.Set(0)
	a.st.copies.Set(1)
	a.st.selection.Set(a.st.selection.Get() + 1)
	a.st.screen.Set(ScreenStudio)

	// Wer Fotos zum Drucken auswählt, hat sie gesehen. Der Eingang ist
	// danach wieder das, was neu ist.
	if err := a.dev.Photos.MarkSeen(a.dev.Subject(), ids...); err != nil {
		a.fail(err)
	}

	a.photosChanged()
}

// photosChanged meldet allen Listen, dass sich Fotos geändert haben.
func (a *App) photosChanged() {
	a.st.photos.Set(a.st.photos.Get() + 1)
}

func toAssetIDs(ids []photo.ID) []asset.ID {
	out := make([]asset.ID, 0, len(ids))
	for _, id := range ids {
		out = append(out, asset.ID(id))
	}

	return out
}

func toPhotoIDs(ids []asset.ID) []photo.ID {
	out := make([]photo.ID, 0, len(ids))
	for _, id := range ids {
		out = append(out, photo.ID(id))
	}

	return out
}
