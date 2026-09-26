package xgift

import (
	"strconv"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"
)

// Filled is a button with a solid face, the primary action of a screen.
//
// Every state keeps the corner radius; the pressed state darkens the face so
// that a touch is visible even though there is no hover on a touchscreen.
func Filled(label gift.View, face ui.Color, radius float32, action func()) ui.ButtonView {
	return ui.Button(label, action).
		Style(ui.ButtonStyle{Background: face, CornerRadius: radius}).
		HoverStyle(ui.ButtonStyle{Background: face, CornerRadius: radius}).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(face, 0.75), CornerRadius: radius}).
		DisabledStyle(ui.ButtonStyle{Background: ui.Fade(face, 0.35), CornerRadius: radius}).
		MinHeight(ui.ControlHitTarget)
}

// Plain is a button that is only its label, for secondary actions and links.
func Plain(label gift.View, action func()) ui.ButtonView {
	clear := ui.ButtonStyle{Background: ui.ColorClear, Border: ui.Border{Width: 1, Color: ui.ColorClear}}
	return ui.Button(label, action).
		Style(clear).
		HoverStyle(clear).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.06), CornerRadius: 8}).
		MinHeight(ui.ControlHitTarget)
}

// Chip is a pill shaped toggle, for filters and small single choices.
func Chip(label string, selected bool, fontSize float32, accent ui.Color, onTap func()) ui.ButtonView {
	fg, bg := ui.ColorLabel, ui.ColorSurface
	border := ui.Border{Width: 1, Color: ui.ColorSeparator}
	if selected {
		fg, bg, border = ui.ColorOnAccent, accent, ui.Border{}
	}

	style := ui.ButtonStyle{Background: bg, Border: border, CornerRadius: 999}

	return ui.Button(ui.Text(label).FontSize(fontSize).Foreground(fg), onTap).
		Style(style).
		HoverStyle(style).
		PressedStyle(ui.ButtonStyle{Background: ui.Fade(accent, 0.3), CornerRadius: 999}).
		PaddingInsets(geom.Insets{Top: 6, Bottom: 6, Left: fontSize, Right: fontSize}).
		MinHeight(ui.ControlHitTarget)
}

// Stepper is a minus/value/plus control for small counts.
func Stepper(value, lo, hi int, fontSize float32, onChange func(int)) gift.View {
	step := func(d int) func() {
		return func() {
			if v := value + d; v >= lo && v <= hi {
				onChange(v)
			}
		}
	}

	btn := func(sym string, d int, enabled bool) gift.View {
		return Plain(ui.Text(sym).FontSize(fontSize*1.3), step(d)).
			Disabled(!enabled).
			Frame(ui.ControlHitTarget*1.2, ui.ControlHitTarget).
			Label(map[int]string{-1: "weniger", 1: "mehr"}[d])
	}

	return ui.HStack(
		btn("−", -1, value > lo),
		ui.Text(strconv.Itoa(value)).FontSize(fontSize).MinWidth(fontSize*2).Align(ui.AlignCenter),
		btn("+", 1, value < hi),
	).Align(geom.Center).Background(ui.ColorControl).CornerRadius(10)
}

// IconTile is an icon on a rounded, coloured square, as settings lists and
// home screens use them.
func IconTile(sym ui.Symbol, face ui.Color, edge float32) gift.View {
	return ui.ZStack(
		ui.Box().Frame(edge, edge).Background(face).CornerRadius(edge*0.23),
		ui.Icon(sym).Size(edge*0.58).Foreground(ui.OpaqueWhite),
	).Align(geom.Center).Frame(edge, edge)
}
