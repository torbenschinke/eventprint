// Package photo enthält die Domäne rund um aufgenommene bzw. hochgeladene
// Fotos einer Fotobox.
//
// Ein Foto ist ein Aggregat, das lediglich Metadaten hält. Die eigentlichen
// Bilddaten liegen im Nago-Image-Subsystem (application/image), welches
// automatisch verkleinerte Varianten (SrcSet) erzeugt und über den
// /api/nago/v1/image Endpunkt ausliefert.
package photo
