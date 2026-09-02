package printing

// TemplateID identifiziert ein Druck-Layout.
type TemplateID string

const (
	// TemplateFull druckt formatfüllend. Das Bild wird mittig auf das
	// Seitenverhältnis des Papiers beschnitten, es bleibt kein weißer Rand.
	TemplateFull TemplateID = "full"

	// TemplateBorder druckt das vollständige Bild mit einem weißen Rand
	// ringsum. Es geht nichts vom Motiv verloren.
	//
	// Der Einzug ist auf allen vier Seiten gleich, der sichtbare Rand nicht:
	// Das Motiv behält sein Seitenverhältnis, und was an den beiden übrigen
	// Kanten frei bleibt, kommt dort hinzu.
	TemplateBorder TemplateID = "border"

	// TemplatePolaroid druckt im Sofortbild-Look: schmaler Rand oben und an
	// den Seiten, breiter Steg unten für eine handschriftliche Notiz.
	TemplatePolaroid TemplateID = "polaroid"
)

// Template beschreibt ein Druck-Layout für die Oberfläche.
type Template struct {
	ID          TemplateID
	Name        string
	Description string
}

// Templates liefert alle auswählbaren Layouts in Anzeigereihenfolge.
func Templates() []Template {
	return []Template{
		{
			ID:          TemplateFull,
			Name:        "Formatfüllend",
			Description: "Randlos über das ganze Papier. Die Ränder des Motivs werden passend beschnitten.",
		},
		{
			ID:          TemplateBorder,
			Name:        "Mit Rand",
			Description: "Das ganze Bild ist zu sehen, nichts wird abgeschnitten. Ringsum wird ein weißer Rand hinzugefügt.",
		},
		{
			ID:          TemplatePolaroid,
			Name:        "Polaroid",
			Description: "Sofortbild-Look mit breitem weißem Steg unten – Platz für einen Gruß.",
		},
	}
}

// TemplateByID liefert das Layout zur ID. Ist die ID unbekannt, wird
// [TemplateFull] zurückgegeben, damit ein Druck niemals an einer veralteten
// Auswahl scheitert.
func TemplateByID(id TemplateID) Template {
	for _, tpl := range Templates() {
		if tpl.ID == id {
			return tpl
		}
	}

	return Templates()[0]
}
