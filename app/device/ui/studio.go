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
	items := res.Value().items

	// Wie die Werkzeugleisten von iPadOS 26: Zurück ist eine runde
	// Glastaste, der Titel ist Text und nie Glas, und die Aktionen teilen
	// sich eine Kapsel.
	h := pick(46, 40)
	glassIcon := func(sym ui.Symbol, label string, fg ui.Color, fn func()) gift.View {
		return ui.Button(ui.Icon(sym).Size(u(20)).Foreground(fg), fn).
			Style(clearButton).HoverStyle(clearButton).
			PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.1), CornerRadius: capsule(h)}).
			Lift(true).
			Frame(u(h), u(h)).Label(label)
	}

	back := glassIcon(outline.ChevronLeft, "Zurück zu den Fotos", ui.ColorLabel, func() {
		a.selected = append([]photo.ID(nil), a.studio...)
		a.st.screen.Set(ScreenLibrary)
	})
	add := glassIcon(outline.Plus, "Fotos hinzufügen", ui.ColorLabel, func() {
		a.selected = append([]photo.ID(nil), a.studio...)
		a.openLibraryKeep(photo.ScopeAll)
	})
	clearAll := glassIcon(outline.TrashBin, "Leeren", red, func() {
		a.studio = nil
		st.selection.Set(st.selection.Get() + 1)
		st.screen.Set(ScreenHome)
	})

	group := func(items ...gift.View) gift.View {
		return glassPane(capsule(h), paneTint(), ui.HStack(items...).Align(geom.Center))
	}

	header := ui.HStack(
		group(back),
		title(fmt.Sprintf("Drucken · %d %s", len(a.studio), plural(len(a.studio), "Foto", "Fotos")), pick(20, 17)).MaxLines(1),
		fill(),
		group(add, clearAll),
	).Gap(u(14)).Align(geom.Center).MinHeight(u(studioHeader()))

	pad := u(pick(22, 10))
	return ui.ZStack(
		xgift.Fill(gift.Component("stage", func(ctx *gift.Context) gift.View { return a.stage(ctx, st, items) })),
		xgift.Fill(ui.VStack(
			header,
			xgift.HStretch(
				a.filmstrip(ctx, st, items),
				grow(gift.Component("preview", func(ctx *gift.Context) gift.View { return a.studioPreview(ctx, st) })),
				xgift.Fill(gift.Component("inspector", func(ctx *gift.Context) gift.View { return a.inspector(ctx, st, sheets) })).Width(u(inspectorWidth())),
			).Gap(u(pick(18, 10))).Flex(1),
		).Gap(u(pick(12, 8))).PaddingInsets(geom.Insets{Left: pad, Right: pad, Top: u(statusBarHeight() + pick(4, 2)), Bottom: pad})),
	).Flex(1)
}

// stage ist der Hintergrund des Studios: das aktive Foto, weichgezeichnet
// und abgedunkelt, damit das Blatt davor das Hellste ist.
func (a *App) stage(ctx *gift.Context, st *states, items []photo.Location) gift.View {
	active := ctx.Read(st.active)
	path := ""
	if active >= 0 && active < len(items) {
		path = items[active].Path
	}

	tone := wallStage
	if !a.dark {
		tone = wallLight
	}

	res := xgift.UseResource[*xgift.MemorySource](ctx, "stage")
	res.LoadKeyed([2]any{path, tone}, func() (*xgift.MemorySource, error) { return wallpaperOf(path, tone), nil })

	if res.Value() == nil {
		return ui.Box().Background(canvas)
	}

	return ui.Image(res.Value()).Fit(ui.FitCover).Placeholder(canvas)
}

// clearButton ist ein Knopf ohne eigene Fläche, etwa in einer Glaskapsel.
var clearButton = ui.ButtonStyle{Background: ui.ColorClear, Border: noBorder}

// Die Maße des Druck-Studios. Die Vorschau bekommt, was Filmstreifen und
// Gestaltung übrig lassen.
func studioHeader() float32   { return pick(52, 42) }
func filmstripWidth() float32 { return pick(116, 80) }
func inspectorWidth() float32 { return pick(380, clamp(vw()*0.4, 280, 340)) }

// filmThumb ist die Kante eines Bildes im Filmstreifen: die Breite ohne den
// Innenrand und den Rahmen des aktiven Bildes.
func filmThumb() float32 { return filmstripWidth() - 2*pick(12, 8) - 2*4 }

// statusBarHeight ist die Höhe der Statuszeile über allen Bildschirmen des
// Heimbetriebs.
func statusBarHeight() float32 { return pick(34, 28) }

// paperSize ist die Größe des Vorschaublattes im Verhältnis 2 : 3, so groß
// wie der Platz zwischen Filmstreifen und Gestaltung es erlaubt.
func paperSize(landscape bool) (float32, float32) {
	availW := vw() - filmstripWidth() - inspectorWidth() - 2*pick(32, 16)
	availH := vh() - statusBarHeight() - studioHeader() - pick(80, 52)

	long := min(float32(450), max(availH, 120))
	if landscape {
		long = min(float32(450), availW, availH*1.5)
	} else {
		long = min(long, availW*1.5)
	}

	short := long * 2 / 3
	if landscape {
		return long, short
	}

	return short, long
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
		img := thumb(it.Path, u(filmThumb())).CornerRadius(u(pick(14, 10)))
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
	}).Frame(u(filmThumb()), u(filmThumb())))

	return xgift.Fill(glassPane(u(pick(28, 20)), paneTint(),
		ui.VScroll(ui.VStack(cells...).Gap(u(pick(12, 8))).Align(geom.Top).Padding(u(pick(12, 8)))).Flex(1),
	)).Width(u(filmstripWidth()))
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

	landscape := false
	if len(res.Value()) > 0 && res.Loaded() {
		pw, ph := a.studioDims(ids)
		landscape = previewLandscape(layout, pw, ph)
	}

	pw, ph := paperSize(landscape)
	w, h := u(pw), u(ph)

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
		fill(),
		ui.VStack(paper).Shadow(ui.Shadow{Blur: u(pick(48, 24)), OffsetY: u(pick(18, 10)), Color: ui.RGBA(0, 0, 0, 110)}),
		glassPill(u(pick(32, 28)), ui.Text(label).FontSize(u(pick(13, 12))).Foreground(ui.ColorSecondaryLabel)),
		fill(),
	).Gap(u(pick(18, 8))).Align(geom.Center).Flex(1)
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

	stepper := xgift.Stepper(copies, 1, printing.MaxCopies, u(16), st.copies.Set)
	finishes := ui.SegmentedControl(finish, []string{"Glanz", "Matt"}, func(i int) {
		set(func(l *printing.Layout) {
			l.Finish = printing.FinishGlossy
			if i == 1 {
				l.Finish = printing.FinishMatte
			}
		})
	}).Capsule(true).FontSize(u(14)).Frame(u(pick(170, 130)), u(36))

	design := glassPane(u(pick(30, 20)), paneTint(),
		ui.SegmentedControl(tab, []string{"Format", "Design", "Bild", "Text"}, st.studioTab.Set).Capsule(true).
			FontSize(u(pick(15, 14))).Frame(geom.Unbounded(), u(pick(40, 36))).Key("tabs"),
		ui.VScroll(content).Flex(1),
	).Gap(u(pick(10, 6))).Padding(u(pick(16, 10))).Flex(1)

	// Die Druckleiste: Anzahl, Oberfläche und der Knopf, eine Glasfläche für
	// sich unter der Gestaltung. Auf kleinen Panels stecken die
	// Beschriftungen in den Bedienelementen selbst.
	var bar ui.Stack
	if compact {
		bar = glassPane(u(20), paneTint(),
			ui.HStack(stepper, fill(), finishes).Align(geom.Center),
			xgift.Fill(primary(printLabel, func() { a.print(st) })).Height(u(44)),
		).Gap(u(8)).Padding(u(10))
	} else {
		bar = glassPane(u(30), paneTint(),
			ui.HStack(stepper, fill(), finishes).Align(geom.Center),
			xgift.Fill(primary(printLabel, func() { a.print(st) })).Height(u(52)),
			muted(fmt.Sprintf("Danach noch etwa %d Blatt im Drucker", max(0, paper-sheets)), 13).Align(ui.AlignCenter),
		).Gap(u(10)).Padding(u(14))
	}

	return ui.VStack(design, bar).Gap(u(pick(14, 8)))
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
	if !solid {
		// Auf Glas sind die Karten selbst nur ein Hauch.
		border = ui.Border{Width: 1, Color: paneEdge()}
		face = ui.Fade(ui.ColorLabel, 0.05)
	}

	if selected {
		border = ui.Border{Width: u(2), Color: blue}
		face = blueWash
		if !solid {
			face = ui.Fade(blue, 0.28)
		}
	}

	style := ui.ButtonStyle{Background: face, Border: border, CornerRadius: u(14)}

	return ui.Button(ui.VStack(
		title(name, pick(15, 14)).MaxLines(1),
		muted(hint, 12).MaxLines(1),
	).Gap(u(2)).Align(geom.Leading), fn).
		Style(style).HoverStyle(style).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(blue, 0.15), Border: border, CornerRadius: u(14)}).
		Lift(true).
		PaddingInsets(geom.Insets{Left: u(pick(14, 10)), Right: u(pick(14, 10)), Top: u(pick(12, 5)), Bottom: u(pick(12, 5))}).
		MinHeight(u(pick(72, 50))).
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

	rows := []gift.View{
		xgift.Grid(2, u(10), cards...),
		ui.HStack(frames...).Gap(u(8)).Align(geom.Center),
	}

	// Die Breite des Passepartouts: 1 cm ist der Rand des Kiosks, schmaler
	// wirkt ein Blatt eleganter.
	if l.Design == printing.DesignMatte {
		mats := []gift.View{body("Randbreite", 15).Flex(1)}
		for _, m := range printing.Mats() {
			mats = append(mats, xgift.Chip(m.Name, l.Mat.Is(m.ID), u(15), blue, func() { set(func(l *printing.Layout) { l.Mat = m.ID }) }))
		}

		rows = append(rows, ui.HStack(mats...).Gap(u(8)).Align(geom.Center))
	}

	return ui.VStack(append(rows,
		ui.List(
			ui.Row("Datumsstempel").Subtitle("Datum orange in der Ecke, wie früher").
				Accessory(ui.Toggle(l.DateStamp, func(v bool) { set(func(l *printing.Layout) { l.DateStamp = v }) })),
		),
	)...).Gap(u(14)).PaddingInsets(geom.Insets{Top: u(12)})
}

func (a *App) imageTab(l printing.Layout, set func(func(*printing.Layout))) gift.View {
	l = l.Normalized()
	chips := []gift.View{}
	for _, f := range printing.Filters() {
		chips = append(chips, xgift.Chip(f.Name, l.Filter == f.ID, u(15), blue, func() { set(func(l *printing.Layout) { l.Filter = f.ID }) }))
	}

	return ui.VStack(
		muted("Farbanmutung", 13),
		xgift.Grid(int(pick(3, 2)), u(8), chips...),
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

	var hint string
	switch {
	case l.Format == printing.FormatPassport:
		hint = "Passfotos bleiben ohne Beschriftung."
	case l.Design == printing.DesignPolaroid:
		hint = "Steht im breiten Steg des Polaroids."
	case l.Design == printing.DesignGallery:
		hint = "Steht unter dem Bild wie ein Schild im Museum."
	case l.Design == printing.DesignMatte:
		hint = "Steht im unteren Rand des Passepartouts."
	default:
		hint = "Randlos und Film haben keinen Platz für Text – wähle Polaroid, Galerie oder Passepartout."
	}

	return ui.VStack(
		body("Beschriftung", 15),
		field(ui.TextField(ed).
			Placeholder("z. B. Sommerfest 2026").
			FontSize(u(17)).
			OnChange(func(v string) { set(func(l *printing.Layout) { l.Caption = v }) }).
			MinHeight(u(48))),
		muted(hint, 13),
		body("Schrift", 15),
		ui.HScroll(fonts...).Gap(u(8)),
		ui.Box().Frame(1, ui.OnScreenKeyboardHeight()),
	).Gap(u(10)).PaddingInsets(geom.Insets{Top: u(12)})
}
