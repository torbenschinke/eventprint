// Package nas holt Fotos von einer Netzwerkfreigabe im Heimnetz, etwa von
// einem Synology-NAS.
//
// Gesprochen wird SMB mit einem reinen Go-Client. Die Freigabe wird nicht
// eingehängt: kein mount, kein root, keine polkit-Regel, und ein NAS, das
// schlafen geht, hinterlässt keinen hängenden Einhängepunkt, an dem der
// Dienst beim Beenden festhinge. Das Gerät liest nur; es öffnet Dateien
// ausschließlich zum Lesen und legt auf dem NAS nichts an.
package nas
