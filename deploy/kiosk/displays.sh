#!/usr/bin/env bash
# Richtet die Bildschirme der Kiosk-Sitzung unter labwc ein: den Touchscreen
# drehen, einen Fernseher spiegeln.
#
# Warum Wayland und nicht mehr X11: Auf dem Raspberry Pi 5 rendert die GPU
# (V3D), das DSI-Panel haengt aber am RP1. X11 kann dort nicht auf das fertige
# Bild umschalten, sondern kopiert jedes Bild in den laufenden Bildschirm.
# Beim Scrollen riss das Bild deshalb sichtbar. labwc schaltet zum Bildwechsel
# um, ohne Riss.
#
# wlroots kennt keinen Clone-Modus wie xrandr --same-as. Gespiegelt wird
# trotzdem: Der Fernseher liegt auf derselben Position wie der Touchscreen
# und ist so skaliert, dass er dieselbe Flaeche zeigt. labwc zeichnet auf
# jedem Ausgang, was auf seiner Flaeche liegt – auf beiden also die Fotobox.
#
# Der Fernseher darf beim Hochfahren fehlen und spaeter dazukommen: Das Skript
# laeuft als Schleife und richtet sich nach dem, was gerade angeschlossen ist.
set -uo pipefail

# Der Touchscreen ist der fuehrende Ausgang. Seine Aufloesung gibt den Ton an,
# denn auf ihm wird bedient.
PRIMARY="${EVENTPRINT_PRIMARY_OUTPUT:-}"

# INTERVAL ist der Takt, in dem nach neuen Bildschirmen gesehen wird.
INTERVAL="${EVENTPRINT_DISPLAY_POLL:-5}"

# ROTATE dreht den Touchscreen. "auto" dreht ein hochkant gebautes Panel ins
# Querformat: Das Raspberry Pi Touch Display 2 (5 und 7 Zoll) meldet
# 720x1280, die Box steht aber quer. "right" ist am Panel in der Halterung der
# Box ausprobiert. Wer das Panel andersherum einbaut, setzt in
# /etc/default/eventprint EVENTPRINT_ROTATE=left. Sonst: normal, left, right,
# inverted – die Namen von xrandr, damit alte Einstellungen weiter gelten.
ROTATE="${EVENTPRINT_ROTATE:-auto}"

# DENSITY erzwingt die Pixeldichte der Oberflaeche (1 oder 2). Leer heisst:
# Die Fotobox bemisst sich selbst.
DENSITY="${EVENTPRINT_UI_DENSITY:-}"

log() {
  if command -v logger >/dev/null 2>&1; then
    logger -t eventprint-kiosk-display "$*"
  fi

  printf '[kiosk-display] %s\n' "$*"
}

outputs_json() {
  wlr-randr --json 2>/dev/null || printf '[]'
}

# transform_for uebersetzt eine Drehung in die Sprache von wlroots. Die
# Richtungen sind gegenlaeufig: xrandr "right" ist wlroots "270".
transform_for() {
  case "$1" in
    left) printf '90' ;;
    right) printf '270' ;;
    inverted) printf '180' ;;
    *) printf 'normal' ;;
  esac
}

apply() {
  local json names primary o mode w h rot transform lw lh mw scale
  json="$(outputs_json)"
  mapfile -t names < <(jq -r '.[].name' <<<"${json}")

  if [[ ${#names[@]} -eq 0 ]]; then
    return 0
  fi

  # Ohne Vorgabe fuehrt ein DSI-Panel – Fernseher haengen an HDMI –, sonst
  # der erste Ausgang.
  primary="${PRIMARY}"
  if [[ -z "${primary}" ]] || ! printf '%s\n' "${names[@]}" | grep -qx "${primary}"; then
    primary="${names[0]}"
    for o in "${names[@]}"; do
      if [[ "${o}" == DSI* ]]; then
        primary="${o}"
        break
      fi
    done
  fi

  mode="$(jq -r --arg o "${primary}" '.[] | select(.name == $o) | ((.modes | map(select(.preferred))) + .modes)[0] | "\(.width)x\(.height)"' <<<"${json}")"
  if [[ ! "${mode}" =~ ^[0-9]+x[0-9]+$ ]]; then
    log "${primary}: kein Modus gemeldet"
    return 0
  fi
  w="${mode%%x*}"
  h="${mode#*x}"

  rot="${ROTATE}"
  if [[ "${rot}" == "auto" ]]; then
    rot="normal"
    if (( h > w )); then
      rot="right"
    fi
  fi
  transform="$(transform_for "${rot}")"

  log "${primary}: ${mode}, gedreht ${rot}"
  wlr-randr --output "${primary}" --on --mode "${mode}" --transform "${transform}" --pos 0,0 --scale 1

  # Die Flaeche, die der Touchscreen zeigt, gedreht gemessen.
  lw="${w}"
  lh="${h}"
  if [[ "${transform}" == "90" || "${transform}" == "270" ]]; then
    lw="${h}"
    lh="${w}"
  fi

  for o in "${names[@]}"; do
    [[ "${o}" != "${primary}" ]] || continue

    mode="$(jq -r --arg o "${o}" '.[] | select(.name == $o) | ((.modes | map(select(.preferred))) + .modes)[0] | "\(.width)x\(.height)"' <<<"${json}")"
    [[ "${mode}" =~ ^[0-9]+x[0-9]+$ ]] || continue
    mw="${mode%%x*}"

    # So skaliert, dass die Breite des Touchscreens die des Fernsehers fuellt.
    # Bei gleichem Seitenverhaeltnis ist das genau dieselbe Flaeche; sonst
    # bleibt am Fernseher unten ein Rand oder es fehlt ein Streifen, die
    # Bedienflaeche auf dem Touchscreen verschiebt sich aber nie.
    scale="$(awk -v a="${mw}" -v b="${lw}" 'BEGIN { printf "%.4f", a / b }')"
    log "spiegele ${o} (${mode}) auf ${primary} (${lw}x${lh}), Skalierung ${scale}"
    wlr-randr --output "${o}" --on --mode "${mode}" --transform normal --pos 0,0 --scale "${scale}"
  done

  if [[ -n "${DENSITY}" ]] && command -v xrdb >/dev/null 2>&1; then
    printf 'Xft.dpi: %s\n' "$(( 96 * DENSITY ))" | xrdb -merge
    log "Xft.dpi $(( 96 * DENSITY ))"
  fi
}

# --once richtet einmal ein und endet. Die Sitzung ruft es vor der Freigabe
# des Bildschirms, damit die Fotobox gleich mit der richtigen Drehung startet.
if [[ "${1:-}" == "--once" ]]; then
  apply
  exit 0
fi

last=""
while true; do
  now="$(outputs_json | jq -r '[.[] | "\(.name):\(.modes | length)"] | join(" ")')"

  # Nur handeln, wenn sich etwas geaendert hat: Jede Umstellung laesst den
  # Bildschirm kurz flackern.
  if [[ "${now}" != "${last}" ]]; then
    log "Bildschirme: ${now:-keine}"
    apply
    last="${now}"
  fi

  sleep "${INTERVAL}"
done
