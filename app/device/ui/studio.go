package uidevice

import (
	"fmt"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/icon/outline"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// studioData sind die Fotos im Druck-Studio.
type studioData struct {
	items []photo.Location
}

// studioScreen ist das Druck-Studio: links die gewählten Fotos, in der Mitte
// die Vorschau, rechts die Gestaltung.
func (a *App) studioScreen(ctx *gift.Context, st *states) gift.View {
	ctx.Read(st.selection)

	if len(a.studio) == 0 {
		return ui.VStack(
			muted("Keine Fotos ausgewählt.", 18),
			primary("Fotos auswählen", func() { a.openLibrary(photo.ScopeAll, "") }),
		).Gap(u(16)).Align(geom.Center).Flex(1)
	}

	res := xgift.UseResource[studioData](ctx, "items")
	res.LoadKeyed(fmt.Sprint(a.studio), func() (studioData, error) {
		locs, err := a.dev.Photos.Locate(a.dev.Subject(), a.studio...)
		return studioData{items: locs}, err
	})

	layout := ctx.Read(st.layout)
	sheets := layout.Sheets(len(a.studio)) * ctx.Read(st.copies)

	header := ui.HStack(
		link("‹ Fotos", func() {
			a.selected = append([]photo.ID(nil), a.studio...)
			a.st.screen.Set(ScreenLibrary)
		}),
		title(fmt.Sprintf("Drucken · %d %s", len(a.studio), plural(len(a.studio), "Foto", "Fotos")), 17).Flex(1).Align(ui.AlignCenter),
		link("Leeren", func() {
			a.studio = nil
			st.selection.Set(st.selection.Get() + 1)
			st.screen.Set(ScreenHome)
		}),
	).Gap(u(12)).Align(geom.Center).PaddingInsets(geom.Insets{Left: u(16), Right: u(16)}).MinHeight(u(56)).
		Background(ui.ColorSurface)

	return ui.VStack(
		header,
		xgift.Hairline(),
		xgift.HStretch(
			a.filmstrip(ctx, st, res.Value().items),
			grow(gift.Component("preview", func(ctx *gift.Context) gift.View { return a.studioPreview(ctx, st) })),
			xgift.Fill(gift.Component("inspector", func(ctx *gift.Context) gift.View { return a.inspector(ctx, st, sheets) })).Width(u(380)),
		).Flex(1),
	).Flex(1)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}

	return many
}

// filmstrip zeigt die Fotos des Druckvorgangs; ein Tipp wählt das Foto, das
// die Vorschau zeigt.
func (a *App) filmstrip(ctx *gift.Context, st *states, items []photo.Location) gift.View {
	active := ctx.Read(st.active)

	cells := []gift.View{}
	for i, it := range items {
		img := thumb(it.Path, u(88))
		if i == active {
			img = img.Border(ui.Border{Width: u(4), Color: blue})
		}

		clear := ui.ButtonStyle{Background: ui.ColorClear, Border: noBorder}
		cells = append(cells, ui.Button(img, func() { st.active.Set(i) }).Style(clear).HoverStyle(clear).PressedStyle(clear).
			Label(fmt.Sprintf("Foto %d", i+1)))
	}

	cells = append(cells, iconButton(outline.Plus, "Fotos hinzufügen", ui.ColorAccent, func() {
		a.selected = append([]photo.ID(nil), a.studio...)
		a.openLibraryKeep(photo.ScopeAll)
	}).Frame(u(88), u(88)))

	return xgift.Fill(ui.VScroll(ui.VStack(cells...).Gap(u(12)).Align(geom.Top).Padding(u(16))).
		Background(ui.ColorBackground)).Width(u(124))
}

// openLibraryKeep öffnet die Mediathek, ohne die Auswahl zu verwerfen.
func (a *App) openLibraryKeep(scope photo.Scope) {
	a.st.libScope.Set(scope)
	a.st.libSource.Set("")
	a.st.selection.Set(a.st.selection.Get() + 1)
	a.st.screen.Set(ScreenLibrary)
}

// sheetPhotos liefert die Fotos des Blattes, auf dem das aktive Foto liegt.
func sheetPhotos(ids []photo.ID, layout printing.Layout, active int) []photo.ID {
	m := max(layout.Motifs(), 1)
	start := (min(max(active, 0), len(ids)-1) / m) * m
	return ids[start:min(start+m, len(ids))]
}

// studioPreview rendert das Blatt mit dem Renderer des Druckers.
func (a *App) studioPreview(ctx *gift.Context, st *states) gift.View {
	layout := ctx.Read(st.layout)
	active := ctx.Read(st.active)
	ctx.Read(st.selection)

	ids := sheetPhotos(a.studio, layout, active)
	res := xgift.UseResource[[]byte](ctx, "preview")
	res.LoadKeyed(fmt.Sprint(layout, ids), func() ([]byte, error) {
		return a.dev.Printing.Preview(a.dev.Subject(), printing.PreviewCmd{Photos: ids, Layout: layout, MaxEdge: int(u(900))})
	})

	w, h := u(300), u(450)
	if len(res.Value()) > 0 && res.Loaded() {
		if pw, ph := a.studioDims(ids); previewLandscape(layout, pw, ph) {
			w, h = h, w
		}
	}

	var paper gift.View = ui.Box().Frame(w, h).Background(white)
	if data := res.Value(); len(data) > 0 {
		paper = ui.Image(xgift.Memory(data)).Fit(ui.FitContain).Frame(w, h)
	}

	m := max(layout.Motifs(), 1)
	sheet := active/m + 1
	of := layout.Sheets(len(a.studio))

	label := fmt.Sprintf("Vorschau · Blatt %d von %d", sheet, of)
	if res.Loading() {
		label = "Vorschau wird berechnet …"
	}

	if res.Err() != nil {
		label = res.Err().Error()
	}

	return ui.VStack(
		muted(label, 14),
		ui.VStack(paper).Shadow(ui.Shadow{Blur: u(36), OffsetY: u(14), Color: ui.RGBA(0, 0, 0, 70)}),
	).Gap(u(16)).Align(geom.Center).Flex(1).Background(ui.RGB(0xE9, 0xE9, 0xEE))
}

// studioDims liefert die Maße des ersten Fotos eines Blattes.
func (a *App) studioDims(ids []photo.ID) (int, int) {
	if len(ids) == 0 {
		return 0, 0
	}

	p, ok, err := a.dev.Photos.FindByID(a.dev.Subject(), ids[0])
	if err != nil || !ok {
		return 0, 0
	}

	return p.Width, p.Height
}

func previewLandscape(l printing.Layout, w, h int) bool {
	return printing.SheetLandscape(l.Normalized(), w, h)
}

// inspector ist die rechte Spalte mit Format, Design, Bild und Text.
func (a *App) inspector(ctx *gift.Context, st *states, sheets int) gift.View {
	tab := ctx.Read(st.studioTab)
	layout := ctx.Read(st.layout)
	copies := ctx.Read(st.copies)

	set := func(fn func(*printing.Layout)) {
		l := st.layout.Get()
		fn(&l)
		st.layout.Set(l)
	}

	var content gift.View
	switch tab {
	case 1:
		content = a.designTab(layout, set)
	case 2:
		content = a.imageTab(layout, set)
	case 3:
		content = a.textTab(ctx, layout, set)
	default:
		content = a.formatTab(layout, set)
	}

	sheets = layout.Sheets(len(a.studio)) * copies
	paper := 0
	if s, err := a.dev.Device.LoadSettings(a.dev.Subject()); err == nil {
		paper = s.PaperLeft
	}

	finish := 0
	if layout.Finish == printing.FinishMatte {
		finish = 1
	}

	printLabel := fmt.Sprintf("%d Blatt drucken", sheets)

	return ui.VStack(
		ui.SegmentedControl(tab, []string{"Format", "Design", "Bild", "Text"}, st.studioTab.Set).
			FontSize(u(15)).Frame(geom.Unbounded(), u(40)).Key("tabs"),
		ui.VScroll(content).Flex(1),
		xgift.Hairline(),
		ui.HStack(body("Anzahl je Blatt", 15).Flex(1), xgift.Stepper(copies, 1, printing.MaxCopies, u(16), st.copies.Set)).Align(geom.Center),
		ui.HStack(body("Oberfläche", 15).Flex(1),
			ui.SegmentedControl(finish, []string{"Glanz", "Matt"}, func(i int) {
				set(func(l *printing.Layout) {
					l.Finish = printing.FinishGlossy
					if i == 1 {
						l.Finish = printing.FinishMatte
					}
				})
			}).FontSize(u(14)).Frame(u(170), u(36)),
		).Align(geom.Center),
		xgift.Fill(primary(printLabel, func() { a.print(st) })).Height(u(54)),
		muted(fmt.Sprintf("Danach noch etwa %d Blatt im Drucker", max(0, paper-sheets)), 13).Align(ui.AlignCenter),
	).Gap(u(12)).Padding(u(16)).Background(ui.ColorSurface)
}

// print gibt den Druckvorgang auf und zeigt den Fortschritt.
func (a *App) print(st *states) {
	batch, err := a.dev.Printing.Print(a.dev.Subject(), printing.PrintCmd{
		Photos: append([]photo.ID(nil), a.studio...),
		Layout: st.layout.Get(),
		Copies: st.copies.Get(),
	})
	if a.fail(err) {
		return
	}

	st.batch.Set(batch.ID)
	a.openSheet(SheetPrinting)
}

// choiceCard ist eine Auswahlkarte der Gestaltung.
func choiceCard(name, hint string, selected bool, fn func()) gift.View {
	border := ui.Border{Width: u(2), Color: ui.ColorSeparator}
	face := ui.ColorSurface
	if selected {
		border = ui.Border{Width: u(2), Color: blue}
		face = ui.RGB(0xEE, 0xF4, 0xFD)
	}

	style := ui.ButtonStyle{Background: face, Border: border, CornerRadius: u(14)}

	return ui.Button(ui.VStack(
		title(name, 15),
		muted(hint, 12),
	).Gap(u(2)).Align(geom.Leading), fn).
		Style(style).HoverStyle(style).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(blue, 0.15), Border: border, CornerRadius: u(14)}).
		PaddingInsets(geom.Insets{Left: u(14), Right: u(14), Top: u(12), Bottom: u(12)}).
		MinHeight(u(72)).
		Align(geom.Leading)
}

func (a *App) formatTab(l printing.Layout, set func(func(*printing.Layout))) gift.View {
	cards := []gift.View{}
	for _, f := range printing.Formats() {
		cards = append(cards, choiceCard(f.Name, f.Hint, l.Normalized().Format == f.ID, func() {
			set(func(l *printing.Layout) { l.Format = f.ID })
			a.st.active.Set(0)
		}))
	}

	return ui.VStack(
		muted("Alle Formate auf 10 × 15 cm Fotopapier", 13),
		xgift.Grid(2, u(10), cards...),
	).Gap(u(10)).PaddingInsets(geom.Insets{Top: u(12)})
}

func (a *App) designTab(l printing.Layout, set func(func(*printing.Layout))) gift.View {
	l = l.Normalized()
	cards := []gift.View{}
	for _, d := range printing.Designs() {
		cards = append(cards, choiceCard(d.Name, d.Hint, l.Design == d.ID, func() { set(func(l *printing.Layout) { l.Design = d.ID }) }))
	}

	frames := []gift.View{body("Rahmenfarbe", 15).Flex(1)}
	for _, f := range printing.Frames() {
		ring := ui.Border{Width: 1, Color: ui.ColorSeparator}
		if l.Frame == f.ID {
			ring = ui.Border{Width: u(3), Color: blue}
		}

		face := ui.ButtonStyle{Background: parseHex(f.Hint, white), Border: ring, CornerRadius: u(20)}
		frames = append(frames, ui.Button(ui.Box().Frame(u(28), u(28)), func() { set(func(l *printing.Layout) { l.Frame = f.ID }) }).
			Style(face).HoverStyle(face).PressedStyle(face).Frame(u(44), u(44)).Label(f.Name))
	}

	return ui.VStack(
		xgift.Grid(2, u(10), cards...),
		ui.HStack(frames...).Gap(u(8)).Align(geom.Center),
		ui.List(
			ui.Row("Datumsstempel").Subtitle("Datum orange in der Ecke, wie früher").
				Accessory(ui.Toggle(l.DateStamp, func(v bool) { set(func(l *printing.Layout) { l.DateStamp = v }) })),
		),
	).Gap(u(14)).PaddingInsets(geom.Insets{Top: u(12)})
}

func (a *App) imageTab(l printing.Layout, set func(func(*printing.Layout))) gift.View {
	l = l.Normalized()
	chips := []gift.View{}
	for _, f := range printing.Filters() {
		chips = append(chips, xgift.Chip(f.Name, l.Filter == f.ID, u(15), blue, func() { set(func(l *printing.Layout) { l.Filter = f.ID }) }))
	}

	return ui.VStack(
		muted("Farbanmutung", 13),
		xgift.Grid(3, u(8), chips...),
		ui.List(
			ui.Row("Ausschnitt auf Gesichter").Subtitle("Erkennt Personen und rückt sie ins Bild").
				Accessory(ui.Toggle(l.FaceCrop, func(v bool) { set(func(l *printing.Layout) { l.FaceCrop = v }) })),
		),
	).Gap(u(12)).PaddingInsets(geom.Insets{Top: u(12)})
}

func (a *App) textTab(ctx *gift.Context, l printing.Layout, set func(func(*printing.Layout))) gift.View {
	ed := ui.Editor(ctx, "caption", l.Caption)
	l = l.Normalized()

	fonts := []gift.View{}
	for _, f := range printing.CaptionFonts() {
		fonts = append(fonts, xgift.Chip(f.Name, l.CaptionFont == f.ID, u(15), blue, func() { set(func(l *printing.Layout) { l.CaptionFont = f.ID }) }))
	}

	hint := "Sichtbar im breiten Steg des Polaroids."
	if l.Design != printing.DesignPolaroid {
		hint = "Die Beschriftung erscheint mit dem Design Polaroid."
	}

	return ui.VStack(
		body("Beschriftung", 15),
		ui.TextField(ed).
			Placeholder("z. B. Sommerfest 2026").
			FontSize(u(17)).
			OnChange(func(v string) { set(func(l *printing.Layout) { l.Caption = v }) }).
			MinHeight(u(48)),
		muted(hint, 13),
		body("Schrift", 15),
		ui.HStack(fonts...).Gap(u(8)),
		ui.Box().Frame(1, ui.OnScreenKeyboardHeight()),
	).Gap(u(10)).PaddingInsets(geom.Insets{Top: u(12)})
}
