package printing

// TemplateID identifiziert ein Druck-Layout.
type TemplateID string

const (
	// TemplateFull druckt formatfüllend. Das Bild wird mittig auf das
	// Seitenverhältnis des Papiers beschnitten, es bleibt kein weißer Rand.
	TemplateFull TemplateID = "full"

	// TemplatePassepartout setzt das Motiv in einen weißen Rahmen von 1 cm,
	// auf allen vier Seiten exakt gleich breit.
	//
	// Anders als die übrigen Layouts hat der Rahmen Vorrang vor dem Motiv:
	// Passt das Seitenverhältnis nicht, wird das Bild beschnitten, statt den
	// Rand ungleichmäßig werden zu lassen.
	//
	// Der gespeicherte Wert bleibt "border". Er steht in bereits abgelegten
	// Druckaufträgen und darf sich deshalb nicht ändern.
	TemplatePassepartout TemplateID = "border"

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
			ID:          TemplatePassepartout,
			Name:        "Passepartout",
			Description: "Das Bild sitzt in einem weißen Rahmen. Der Rand ist überall 1 cm breit. Dafür wird das Bild an den Kanten etwas beschnitten.",
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
