#!/usr/bin/env bash
# Spiegelt den Fernseher auf den Touchscreen.
#
# Warum X11 und nicht Wayland: labwc und wlroots kennen keinen Clone-Modus.
# Zwei Ausgaenge zeigen dort zwangslaeufig verschiedene Ausschnitte, und das
# einzige Werkzeug dagegen (wl-mirror) ist in Raspberry Pi OS nicht paketiert.
# xrandr --same-as kann es seit jeher. Der Fernseher entscheidet also die
# Anzeigeschicht, nicht der Geschmack.
#
# Der Fernseher darf beim Hochfahren fehlen und spaeter dazukommen: Das Skript
# laeuft als Schleife und richtet sich nach dem, was gerade angeschlossen ist.
set -uo pipefail

# Der Touchscreen ist der fuehrende Ausgang. Seine Aufloesung gibt den Ton an,
# denn auf ihm wird bedient - ein Fernseher, der das Layout verschiebt, macht
# die Bedienflaechen unerreichbar.
PRIMARY="${EVENTPRINT_PRIMARY_OUTPUT:-}"

# INTERVAL ist der Takt, in dem nach neuen Bildschirmen gesehen wird. X11
# meldet das Anstecken nicht von sich aus an ein Skript.
INTERVAL="${EVENTPRINT_DISPLAY_POLL:-5}"

# Ins Journal, damit die Spiegelung im Stoerungsfall nachvollziehbar ist. Die
# Ausgabe landete sonst im Heimverzeichnis des Kiosk-Nutzers und war fuer
# niemanden lesbar.
log() {
  if command -v logger >/dev/null 2>&1; then
    logger -t eventprint-kiosk-display "$*"
  fi

  printf '[kiosk-display] %s\n' "$*"
}

# connected_outputs listet die Ausgaenge mit angeschlossenem Geraet.
connected_outputs() {
  xrandr --query | awk '$2 == "connected" { print $1 }'
}

# modes_of listet die Modi eines Ausgangs, beste zuerst.
modes_of() {
  xrandr --query | awk -v out="$1" '
    $1 == out { grab = 1; next }
    grab && $2 == "connected" { exit }
    grab && /^[[:space:]]+[0-9]+x[0-9]+/ { print $1 }
    grab && $0 !~ /^[[:space:]]/ { exit }
  '
}

# ROTATE dreht den Touchscreen. "auto" dreht ein hochkant gebautes Panel ins
# Querformat: Das Raspberry Pi Touch Display 2 (5 und 7 Zoll) meldet
# 720x1280, die Box steht aber quer. "left" entspricht dem rotate=270, das die
# Raspberry-Pi-Dokumentation dafuer nennt; wer das Panel andersherum einbaut,
# setzt EVENTPRINT_ROTATE=right. Sonst: normal, left, right, inverted.
ROTATE="${EVENTPRINT_ROTATE:-auto}"

# DENSITY erzwingt die Pixeldichte der Oberflaeche (1 oder 2); leer heisst
# nach der Groesse des Panels.
DENSITY="${EVENTPRINT_UI_DENSITY:-}"

# rotation_for liefert die Drehung fuer einen Ausgang.
rotation_for() {
  local first w h
  if [[ "${ROTATE}" != "auto" ]]; then
    printf '%s' "${ROTATE}"
    return 0
  fi

  first="$(modes_of "$1" | head -1)"
  w="${first%%x*}"
  h="${first#*x}"
  h="${h%%[!0-9]*}"
  if [[ -n "${w}" && -n "${h}" && "${h}" -gt "${w}" ]]; then
    printf 'left'
  else
    printf 'normal'
  fi
}

# transposed dreht einen Modus "BxH" um, wenn die Drehung Breite und Hoehe
# tauscht.
transposed() {
  local mode="$1" rot="$2" w h
  case "${rot}" in
    left|right)
      w="${mode%%x*}"
      h="${mode#*x}"
      h="${h%%[!0-9]*}"
      printf '%sx%s' "${h}" "${w}"
      ;;
    *) printf '%s' "${mode}" ;;
  esac
}

# mirror_pair sucht die beste Aufloesung, die beide Geraete koennen, und
# liefert "Modus-des-Touchscreens Modus-des-Fernsehers".
#
# Ohne diesen Abgleich waehlt xrandr fuer den Fernseher irgendeinen Modus und
# skaliert oder schneidet ab. Ein gemeinsamer Modus zeigt auf beiden dasselbe
# Bild, unverzerrt. Ein gedrehter Touchscreen zeigt 720x1280 als 1280x720;
# verglichen wird deshalb, was man sieht.
mirror_pair() {
  local primary="$1" secondary="$2" rot="$3" pm seen theirs
  theirs="$(modes_of "${secondary}")"

  while read -r pm; do
    seen="$(transposed "${pm}" "${rot}")"
    if grep -qx "${seen}" <<<"${theirs}"; then
      printf '%s %s' "${pm}" "${seen}"
      return 0
    fi
  done < <(modes_of "${primary}" | sort -u -t x -k1,1nr -k2,2nr)
}

# map_touch legt die Beruehrung auf den Touchscreen. Ohne das rechnet X die
# Finger auf den ganzen Bildschirm um – falsch, sobald das Panel gedreht ist
# oder ein Fernseher mit anderer Aufloesung dazukommt. map-to-output beruecksichtigt
# die Drehung selbst.
map_touch() {
  local out="$1" id
  command -v xinput >/dev/null 2>&1 || return 0

  while read -r id; do
    [[ -n "${id}" ]] || continue
    if xinput map-to-output "${id}" "${out}" 2>/dev/null; then
      log "Touch ${id} auf ${out}"
    fi
  done < <(touch_ids)
}

# touch_ids listet die Touch-Eingabegeraete.
touch_ids() {
  command -v xinput >/dev/null 2>&1 || return 0
  xinput list 2>/dev/null |
    grep -Ei 'touch|ft5x06|goodix|ili2|edt-ft|raspberrypi-ts|waveshare' |
    grep -i 'pointer' |
    sed -n 's/.*id=\([0-9]*\).*/\1/p'
}

# set_density sagt der Oberflaeche ueber Xft.dpi, wie dicht das Panel ist.
#
# gift zeichnet mit ganzzahliger Dichte und liest sie aus Xft.dpi: 96 ist 1,
# 192 ist 2. Auf dem Touch Display 2 mit 1280x720 auf 5 oder 7 Zoll waeren
# die Tasten der Bildschirmtastatur mit Dichte 1 nur 4 bis 5 mm hoch. Ab etwa
# 180 ppi wird deshalb verdoppelt; alles Weitere rechnet die Fotobox selbst.
set_density() {
  local out="$1" line px mm pw ph mw mh ppi dpi=96
  command -v xrdb >/dev/null 2>&1 || return 0

  if [[ -n "${DENSITY}" ]]; then
    dpi=$(( 96 * DENSITY ))
  else
    line="$(xrandr --query | awk -v o="${out}" '$1 == o')"
    px="$(grep -oE '[0-9]+x[0-9]+\+' <<<"${line}" | head -1)"
    mm="$(grep -oE '[0-9]+mm x [0-9]+mm' <<<"${line}" | tail -1)"
    pw="${px%%x*}"; ph="${px#*x}"; ph="${ph%+}"
    mw="${mm%%mm*}"; mh="${mm##*x }"; mh="${mh%mm}"

    if [[ -n "${pw}" && -n "${mw}" && "${mw}" -gt 0 && "${mh}" -gt 0 ]]; then
      # Laengste Seite gegen laengste Seite: Gedrehte Panels melden ihre
      # Masse oft ungedreht.
      (( pw < ph )) && { local t="${pw}"; pw="${ph}"; ph="${t}"; }
      (( mw < mh )) && { local t="${mw}"; mw="${mh}"; mh="${t}"; }
      ppi=$(( pw * 254 / (mw * 10) ))
      if (( ppi >= 180 && ppi <= 420 )); then
        dpi=192
      fi
      log "${out}: ${pw}x${ph} Pixel auf ${mw}x${mh} mm, ${ppi} ppi"
    fi
  fi

  printf 'Xft.dpi: %s\n' "${dpi}" | xrdb -merge
  log "Xft.dpi ${dpi}"
}

apply() {
  local outputs primary secondary pair rot o
  mapfile -t outputs < <(connected_outputs)

  if [[ ${#outputs[@]} -eq 0 ]]; then
    return 0
  fi

  # Der Touchscreen fuehrt. Ohne Vorgabe ist das ein DSI-Panel, falls eines
  # angeschlossen ist – Fernseher haengen an HDMI –, sonst der erste Ausgang.
  primary="${PRIMARY}"
  if [[ -z "${primary}" ]] || ! printf '%s\n' "${outputs[@]}" | grep -qx "${primary}"; then
    primary="${outputs[0]}"
    for o in "${outputs[@]}"; do
      if [[ "${o}" == DSI* ]]; then
        primary="${o}"
        break
      fi
    done
  fi

  rot="$(rotation_for "${primary}")"

  # Nur ein Bildschirm: alles andere abschalten und fertig.
  if [[ ${#outputs[@]} -eq 1 ]]; then
    xrandr --output "${primary}" --auto --rotate "${rot}" --primary
    map_touch "${primary}"
    set_density "${primary}"
    return 0
  fi

  for secondary in "${outputs[@]}"; do
    [[ "${secondary}" != "${primary}" ]] || continue

    pair="$(mirror_pair "${primary}" "${secondary}" "${rot}")"

    if [[ -z "${pair}" ]]; then
      # Kein gemeinsamer Modus: Lieber den Touchscreen unveraendert lassen und
      # den Fernseher automatisch fahren, als die Bedienflaeche zu verschieben.
      log "kein gemeinsamer Modus fuer ${primary} und ${secondary}; Notbehelf"
      xrandr --output "${primary}" --auto --rotate "${rot}" --primary \
             --output "${secondary}" --auto --same-as "${primary}"
      continue
    fi

    log "spiegele ${secondary} auf ${primary} mit ${pair}"
    xrandr --output "${primary}" --mode "${pair% *}" --rotate "${rot}" --primary \
           --output "${secondary}" --mode "${pair#* }" --same-as "${primary}"
  done

  map_touch "${primary}"
  set_density "${primary}"
}

# --once richtet einmal ein und endet. Die Sitzung ruft es vor der Freigabe
# des Bildschirms, damit die Fotobox gleich mit der richtigen Drehung und
# Dichte startet.
if [[ "${1:-}" == "--once" ]]; then
  apply
  exit 0
fi

last=""
while true; do
  # Touch-Geraete gehoeren dazu: Ein USB-Touch meldet sich oft erst nach
  # dem Bildschirm, und dann muss die Zuordnung nachgezogen werden.
  now="$(connected_outputs | sort | tr '\n' ' ')/$(touch_ids | tr '\n' ' ')"

  # Nur handeln, wenn sich etwas geaendert hat. Ein xrandr-Aufruf im Sekunden-
  # takt laesst den Bildschirm sichtbar flackern.
  if [[ "${now}" != "${last}" ]]; then
    log "Bildschirme: ${now:-keine}"
    apply
    last="${now}"
  fi

  sleep "${INTERVAL}"
done
