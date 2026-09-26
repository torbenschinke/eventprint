package xgift

import (
	"strconv"
	"strings"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"
)

// PinPad is a numeric keypad with a row of dots for a fixed length PIN.
//
// It holds no state of its own: the caller keeps the digits typed so far and
// receives every change. When the last digit is typed, onComplete is called
// with the full PIN; clearing the entry afterwards is the caller's decision,
// because only the caller knows whether it was right.
func PinPad(length int, value string, key float32, onChange func(string), onComplete func(string)) gift.View {
	dots := make([]gift.View, 0, length)
	for i := range length {
		fill := ui.ColorClear
		if i < len(value) {
			fill = ui.ColorLabel
		}

		dots = append(dots, ui.Box().Frame(key/4, key/4).CornerRadius(key/8).
			Background(fill).Border(ui.Border{Width: 2, Color: ui.ColorLabel}))
	}

	press := func(d string) func() {
		return func() {
			if len(value) >= length {
				return
			}

			next := value + d
			onChange(next)
			if len(next) == length && onComplete != nil {
				onComplete(next)
			}
		}
	}

	button := func(label string, fn func()) gift.View {
		face := ui.ButtonStyle{Background: ui.ColorControl, CornerRadius: key / 2}
		return ui.Button(ui.Text(label).FontSize(key*0.4), fn).
			Style(face).HoverStyle(face).
			PressedStyle(ui.ButtonStyle{Background: ui.ColorControlPressed, CornerRadius: key / 2}).
			Frame(key, key).
			Label(label)
	}

	rows := []gift.View{}
	for r := range 3 {
		cells := []gift.View{}
		for c := range 3 {
			d := strconv.Itoa(r*3 + c + 1)
			cells = append(cells, button(d, press(d)))
		}

		rows = append(rows, ui.HStack(cells...).Gap(key/3))
	}

	back := Plain(ui.Text("⌫").FontSize(key*0.35), func() {
		if value != "" {
			onChange(value[:len(value)-1])
		}
	}).Frame(key, key).Label("löschen")

	rows = append(rows, ui.HStack(ui.Box().Frame(key, key), button("0", press("0")), back).Gap(key/3))

	return ui.VStack(
		ui.HStack(dots...).Gap(key/5),
		ui.Box().Frame(1, key/4),
		ui.VStack(rows...).Gap(key/4),
	).Align(geom.Center).Key("pinpad-" + strings.Repeat("•", len(value)))
}
