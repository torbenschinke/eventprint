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

log() { printf '[kiosk-display] %s\n' "$*"; }

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

# common_mode sucht die beste Aufloesung, die beide Geraete koennen.
#
# Ohne diesen Abgleich waehlt xrandr fuer den Fernseher irgendeinen Modus und
# skaliert oder schneidet ab. Ein gemeinsamer Modus zeigt auf beiden dasselbe
# Bild, unverzerrt.
common_mode() {
  local a="$1" b="$2"

  comm -12 \
    <(modes_of "$a" | sort -u) \
    <(modes_of "$b" | sort -u) |
    sort -t x -k1,1nr -k2,2nr |
    head -1
}

apply() {
  local outputs primary secondary mode
  mapfile -t outputs < <(connected_outputs)

  if [[ ${#outputs[@]} -eq 0 ]]; then
    return 0
  fi

  primary="${PRIMARY}"
  if [[ -z "${primary}" ]] || ! printf '%s\n' "${outputs[@]}" | grep -qx "${primary}"; then
    primary="${outputs[0]}"
  fi

  # Nur ein Bildschirm: alles andere abschalten und fertig.
  if [[ ${#outputs[@]} -eq 1 ]]; then
    xrandr --output "${primary}" --auto --primary
    return 0
  fi

  for secondary in "${outputs[@]}"; do
    [[ "${secondary}" != "${primary}" ]] || continue

    mode="$(common_mode "${primary}" "${secondary}")"

    if [[ -z "${mode}" ]]; then
      # Kein gemeinsamer Modus: Lieber den Touchscreen unveraendert lassen und
      # den Fernseher automatisch fahren, als die Bedienflaeche zu verschieben.
      log "kein gemeinsamer Modus fuer ${primary} und ${secondary}; Notbehelf"
      xrandr --output "${primary}" --auto --primary \
             --output "${secondary}" --auto --same-as "${primary}"
      continue
    fi

    log "spiegele ${secondary} auf ${primary} mit ${mode}"
    xrandr --output "${primary}" --mode "${mode}" --primary \
           --output "${secondary}" --mode "${mode}" --same-as "${primary}"
  done
}

last=""
while true; do
  now="$(connected_outputs | sort | tr '\n' ' ')"

  # Nur handeln, wenn sich etwas geaendert hat. Ein xrandr-Aufruf im Sekunden-
  # takt laesst den Bildschirm sichtbar flackern.
  if [[ "${now}" != "${last}" ]]; then
    log "Bildschirme: ${now:-keine}"
    apply
    last="${now}"
  fi

  sleep "${INTERVAL}"
done
