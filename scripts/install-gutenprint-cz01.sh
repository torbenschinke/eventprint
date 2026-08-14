#!/usr/bin/env bash
set -euo pipefail

GUTENPRINT_VERSION="5.3.4.20220624T01008808d602"
ARCHIVE="gutenprint_${GUTENPRINT_VERSION}.orig.tar.xz"
SOURCE_URL="https://deb.debian.org/debian/pool/main/g/gutenprint/${ARCHIVE}"
SOURCE_SHA256="ef514d4ca567b5871cfe7697127bee2c14d5d71d7fbc68d8fee96b0faaed90e7"
PREFIX="/opt/eventprint/gutenprint"
MODULE_DIR="${PREFIX}/5.3/modules"
FILTER="${PREFIX}/bin/rastertocz01"

if [[ ${EUID} -eq 0 ]]; then
  SUDO=()
else
  SUDO=(sudo)
fi

if ! command -v apt-get >/dev/null; then
  printf 'This installer requires a Debian-based system with apt-get.\n' >&2
  exit 1
fi

"${SUDO[@]}" apt-get update
"${SUDO[@]}" apt-get install -y \
  build-essential ca-certificates cups-filters curl dpkg-dev libcups2-dev \
  libltdl-dev libtool-bin printer-driver-gutenprint python3 xz-utils

installed_version="$(dpkg-query -W -f='${Version}' libgutenprint9 2>/dev/null || true)"
if [[ ${installed_version} != 5.3.4* ]]; then
  printf 'Unsupported system Gutenprint version: %s (expected 5.3.4.x).\n' "${installed_version:-not installed}" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

curl --fail --location --output "${work}/${ARCHIVE}" "${SOURCE_URL}"
printf '%s  %s\n' "${SOURCE_SHA256}" "${work}/${ARCHIVE}" | sha256sum --check --status

mkdir "${work}/src"
tar -xJf "${work}/${ARCHIVE}" --strip-components=1 -C "${work}/src"

python3 - "${work}/src/src/main/print-dyesub.c" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
source = path.read_text()
pattern = re.compile(
    r'(DEFINE_PAPER\(\s*"w288h432",\s*"4x6",\s*'
    r'PT1?\(1408,300\),\s*PT1?\(1836,300\),\s*)'
    r'PT\(71,300\),\s*PT\(71,300\)'
    r'(,\s*0,\s*0,\s*DYESUB_PORTRAIT\))'
)
source, count = pattern.subn(r'\1PT(92,300), PT(92,300)\2', source)
if count != 1:
    raise SystemExit('Expected CZ-01/QW410 4x6 geometry was not found exactly once')
path.write_text(source)
PY

(
  cd "${work}/src"
  ./configure \
    --with-modules=dlopen \
    --disable-static \
    --disable-libgutenprintui2 \
    --disable-cups-ppds \
    --without-gimp2 \
    --without-cups \
    --without-doc \
    --disable-test \
    --disable-testpattern \
    --disable-escputil
  make -C src/main -j"$(nproc)"
)

"${SUDO[@]}" install -d -m 0755 "${MODULE_DIR}" "$(dirname "${FILTER}")"
"${SUDO[@]}" install -m 0644 "${work}/src/src/main/.libs/print-dyesub.so" "${MODULE_DIR}/print-dyesub.so"

wrapper="${work}/rastertocz01"
cat >"${wrapper}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
multiarch="$(dpkg-architecture -qDEB_HOST_MULTIARCH)"
export STP_MODULE_PATH="/opt/eventprint/gutenprint/5.3/modules:/usr/lib/${multiarch}/gutenprint/5.3/modules"
exec /usr/lib/cups/filter/rastertogutenprint.5.3 "$@"
EOF
"${SUDO[@]}" install -m 0755 "${wrapper}" "${FILTER}"

printf 'Installed CZ-01 Gutenprint filter: %s\n' "${FILTER}"
