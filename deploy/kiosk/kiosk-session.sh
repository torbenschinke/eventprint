#!/usr/bin/env bash
# Sitzung des Kiosk-Nutzers: Bildschirme spiegeln, der Fotobox den Bildschirm
# überlassen.
#
# Die Oberfläche ist seit dem Umstieg auf gift kein Browser mehr, sondern ein
# eigenes Programm, das der Dienst eventprint.service als Nutzer eventprint
# startet. Diese Sitzung stellt nur den Bildschirm bereit: X11 mit Openbox, ohne
# Leiste, ohne Menü, ohne Dateimanager – nichts, worüber ein Gast die Fotobox
# verlassen könnte.
set -uo pipefail

# Alles ins Journal. Ein Kiosk, der unbeaufsichtigt läuft, muss sagen können,
# was ihm fehlt.
if command -v logger >/dev/null 2>&1; then
  exec > >(logger -t eventprint-kiosk) 2>&1
fi

echo "Sitzung startet"

# Kein Bildschirmschoner, kein Abschalten. Die Fotobox steht den Abend über
# ungenutzt herum, und ein schwarzer Bildschirm sieht aus wie ein Defekt.
xset s off
xset s noblank
xset -dpms

# Den Mauszeiger verstecken. Auf einem Touchscreen ist er nur ein Fleck, den
# niemand wegbekommt; die Fotobox blendet ihn zusätzlich selbst aus.
if command -v unclutter >/dev/null 2>&1; then
  unclutter -idle 1 -root &
fi

# Erst einmal einrichten – Drehung, Touch, Dichte –, dann im Hintergrund
# auf neue Bildschirme achten. Die Fotobox liest die Dichte beim Start; sie
# darf den Bildschirm deshalb erst danach bekommen.
/usr/local/bin/eventprint-mirror-displays --once
/usr/local/bin/eventprint-mirror-displays &

# Dem Dienstnutzer den Bildschirm freigeben – genau ihm und niemandem sonst.
# SI:localuser prüft die Kennung des verbindenden Prozesses über den lokalen
# Socket; ein Rechner im Netz kommt so nicht an den Bildschirm.
if xhost "+SI:localuser:eventprint" >/dev/null; then
  echo "Bildschirm für eventprint freigegeben"
else
  echo "xhost fehlgeschlagen - die Fotobox kann nicht zeichnen"
fi

# Die Sitzung muss leben, solange der Bildschirm gebraucht wird. Endet sie,
# beendet lightdm auch den X-Server.
exec sleep infinity
