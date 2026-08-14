#!/usr/bin/env bash
# Druckt ein Bild randlos (Auto-Crop, kein Letterboxing) auf dem Citizen CZ-01 (10x15 cm).
set -euo pipefail

IMAGE="${1:-$(dirname "$(readlink -f "$0")")/DSC02301.jpg}"
PRINTER="CZ01"
DPI=300
# 4x6 Zoll @ 300 dpi
LONG=$((6 * DPI))   # 1800
SHORT=$((4 * DPI))  # 1200

[[ -f "$IMAGE" ]] || { echo "Bild nicht gefunden: $IMAGE" >&2; exit 1; }

# Ausrichtung des Originals bestimmen (EXIF-Rotation beruecksichtigt)
read -r W H < <(magick identify -format "%w %h\n" "$IMAGE[0]")
if (( W >= H )); then
  TARGET="${LONG}x${SHORT}"    # Querformat
else
  TARGET="${SHORT}x${LONG}"    # Hochformat
fi

TMP="$(mktemp --suffix=.jpg)"
trap 'rm -f "$TMP"' EXIT

# auto-orient + formatfuellend skalieren + mittig auf exaktes 4x6-Format beschneiden
magick "$IMAGE" \
  -auto-orient \
  -resize "${TARGET}^" \
  -gravity center \
  -extent "$TARGET" \
  -density "$DPI" -units PixelsPerInch \
  -quality 95 \
  "$TMP"

lp -d "$PRINTER" \
   -o PageSize=w288h432 \
   -o StpImageType=Photo \
   -o fit-to-page \
   -o media=w288h432 \
   -o scaling=100 \
   "$TMP"
