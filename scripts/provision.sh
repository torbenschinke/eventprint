#!/usr/bin/env bash
#
# Gleicht die Systemkonfiguration der Box an das Repository an.
#
# Der Grund, warum es diese Datei gibt: update.sh laeuft unprivilegiert und
# kann ausschliesslich Go-Code ausspielen. Jede Aenderung an udev, systemd oder
# Paketen erforderte deshalb, dass jemand mit root vor der Box sitzt - und beim
# ersten Einsatz hiess das eine Autofahrt, weil die Box im Gastnetz hinter NAT
# stand und nicht erreichbar war.
#
# Dieses Skript laeuft als root beim Hochfahren, vor dem Updater. Damit ist
# Systemkonfiguration per "git push" zustellbar: pushen, jemanden vor Ort den
# Strom ziehen lassen, fertig.
#
# Zwei Regeln wie beim Updater:
#
#   1. Es endet immer mit Erfolg. Es laeuft vor dem Dienst; braeche es ab,
#      startete die Fotobox nicht.
#   2. Es ist wiederholbar und aendert nichts, was schon stimmt.
#
# ACHTUNG: Wer auf diesen Zweig pushen kann, fuehrt hier Befehle als root aus.
# Das ist bewusst so entschieden, damit eine Reparatur aus der Ferne moeglich
# ist, und es setzt voraus, dass der Zugang zum Repository entsprechend eng
# bleibt.

set -uo pipefail

readonly ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

log() { printf '%s eventprint-provision: %s\n' "$(date -Is)" "$*"; }

if [[ "${EUID}" -ne 0 ]]; then
  log "laeuft nicht als root, es gibt nichts zu tun"
  exit 0
fi

changed=0

# ------------------------------------------------------------------- Units ---
# Die systemd-Units werden nach /etc kopiert, nicht verlinkt. Ein "git pull"
# aendert sie deshalb nicht - genau der Henne-Ei-Fehler, der dieses Skript
# noetig gemacht hat, nur eine Ebene hoeher. Ohne diesen Abschnitt waere jede
# Aenderung an einer Unit wieder nur mit root vor dem Geraet zustellbar.
#
# Die eigene Unit wird dabei mitaktualisiert. Das wirkt beim naechsten Start,
# nicht sofort, und das genuegt.
units_changed=0
for unit in eventprint-provision.service eventprint-update.service eventprint.service; do
  src="${ROOT_DIR}/deploy/systemd/${unit}"
  dst="/etc/systemd/system/${unit}"
  [[ -f "${src}" ]] || continue
  if ! cmp -s "${src}" "${dst}"; then
    log "Unit ${unit} angleichen"
    install -m 0644 "${src}" "${dst}"
    units_changed=1
    changed=1
  fi
done
if [[ ${units_changed} -eq 1 ]]; then
  systemctl daemon-reload
  systemctl enable eventprint-provision.service eventprint-update.service eventprint.service >/dev/null 2>&1
fi

# Die polkit-Regeln und die Kiosk-Sitzung stammen aus dem Repository und
# werden angeglichen wie die Units. Vorher brachte sie nur install.sh auf das
# Geraet; eine Aenderung daran hing an einer Neuinstallation – und damit an
# jemandem mit root vor der Box.
if [[ -d /etc/polkit-1/rules.d ]]; then
  for rule in "${ROOT_DIR}"/deploy/polkit/*.rules; do
    [[ -f "${rule}" ]] || continue
    dst="/etc/polkit-1/rules.d/$(basename "${rule}")"
    if ! cmp -s "${rule}" "${dst}"; then
      log "polkit-Regel $(basename "${rule}") angleichen"
      install -m 0644 "${rule}" "${dst}"
      changed=1
    fi
  done
fi

# udisks2 haengt USB-Sticks ein, auf die die Fotos einer Feier kopiert werden.
# Aeltere Installationen haben es nicht. Ohne Netz gelingt die Nachinstallation
# nicht; dann eben beim naechsten Start – die Box startet trotzdem.
#
# Dasselbe gilt fuer libheif, ohne die HEIC-Fotos von iPhones abgelehnt werden,
# und fuer die Kiosk-Sitzung unter labwc: labwc selbst, wlr-randr zum Drehen
# und Spiegeln, jq zum Lesen seiner Ausgabe. Sie stehen vor der Sitzung, weil
# die Anmeldung erst auf labwc umgestellt wird, wenn alles dafuer da ist.
missing=()
command -v udisksctl >/dev/null 2>&1 || missing+=(udisks2)
ldconfig -p 2>/dev/null | grep -q 'libheif\.so\.1' || missing+=(libheif1)
ldconfig -p 2>/dev/null | grep -q 'libturbojpeg\.so\.0' || missing+=(libturbojpeg0)
command -v labwc >/dev/null 2>&1 || missing+=(labwc)
command -v Xwayland >/dev/null 2>&1 || missing+=(xwayland)
command -v wlr-randr >/dev/null 2>&1 || missing+=(wlr-randr)
command -v jq >/dev/null 2>&1 || missing+=(jq)
command -v xhost >/dev/null 2>&1 || missing+=(x11-xserver-utils)
if [[ ${#missing[@]} -gt 0 ]]; then
  log "nachinstallieren: ${missing[*]}"
  if timeout 300 env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${missing[@]}" >/dev/null 2>&1; then
    changed=1
  else
    log "${missing[*]} liess sich nicht installieren; was davon abhaengt, bleibt bis dahin gesperrt"
  fi
fi

# ------------------------------------------------------------------- Kiosk ---
# Skripte und labwc-Konfiguration der Kiosk-Sitzung. Die Konfiguration liegt
# beim Kiosk-Nutzer, weil labwc dort zuerst sucht und eine Datei dort die des
# Systems ersetzt – samt Leiste und Dateimanager, die Raspberry Pi OS unter
# /etc/xdg/labwc startet.
KIOSK_USER="${KIOSK_USER:-fotobox}"
session_changed=0
for script in kiosk-session.sh:eventprint-kiosk-session displays.sh:eventprint-displays; do
  src="${ROOT_DIR}/deploy/kiosk/${script%%:*}"
  dst="/usr/local/bin/${script##*:}"
  [[ -f "${src}" ]] || continue
  if ! cmp -s "${src}" "${dst}"; then
    log "Kiosk-Skript ${script##*:} angleichen"
    install -m 0755 "${src}" "${dst}"
    session_changed=1
    changed=1
  fi
done

kiosk_home="$(getent passwd "${KIOSK_USER}" 2>/dev/null | cut -d: -f6 || true)"
if [[ -n "${kiosk_home}" && -d "${kiosk_home}" ]]; then
  install -d -o "${KIOSK_USER}" -g "${KIOSK_USER}" -m 0755 "${kiosk_home}/.config" "${kiosk_home}/.config/labwc"
  for f in rc.xml environment autostart; do
    src="${ROOT_DIR}/deploy/kiosk/labwc/${f}"
    dst="${kiosk_home}/.config/labwc/${f}"
    if ! cmp -s "${src}" "${dst}"; then
      log "labwc: ${f} angleichen"
      install -o "${KIOSK_USER}" -g "${KIOSK_USER}" -m 0644 "${src}" "${dst}"
      session_changed=1
      changed=1
    fi
  done
fi

# Die automatische Anmeldung muss in die Kiosk-Sitzung fuehren. Raspberry Pi
# OS schreibt sie zurueck, sobald jemand in raspi-config die Anzeigeschicht
# umstellt: Danach meldete sich der Kiosk-Nutzer in den normalen Desktop an
# (autologin-session=rpd-x), die Kiosk-Sitzung lief nicht, und die Fotobox
# bekam keinen Zugriff auf den Bildschirm. Eine Woche lang startete sie alle
# drei Sekunden neu, und auf dem Touchscreen stand ein ungedrehter Desktop.
#
# Die Sitzung ist labwc und nicht mehr Openbox unter X11, siehe
# deploy/kiosk/displays.sh. Umgestellt wird nur, wenn labwc und seine
# Werkzeuge da sind; sonst bliebe der Bildschirm schwarz. Die Hauptdatei
# gewinnt gegen conf.d, siehe configure_autologin in install.sh.
lightdm_conf=/etc/lightdm/lightdm.conf
if [[ -f "${lightdm_conf}" ]] && id -u "${KIOSK_USER}" >/dev/null 2>&1 &&
   command -v labwc >/dev/null 2>&1 && command -v wlr-randr >/dev/null 2>&1 && command -v jq >/dev/null 2>&1; then
  for pair in \
    "autologin-user=${KIOSK_USER}" \
    "autologin-user-timeout=0" \
    "autologin-session=labwc" \
    "user-session=labwc"
  do
    key="${pair%%=*}"
    if grep -qxF "${pair}" "${lightdm_conf}"; then
      continue
    fi

    log "lightdm: ${pair}"
    if grep -qE "^[[:space:]]*${key}=" "${lightdm_conf}"; then
      sed -i -E "s|^[[:space:]]*${key}=.*|${pair}|" "${lightdm_conf}"
    else
      sed -i "0,/^\[Seat:\*\]/s//[Seat:*]\n${pair}/" "${lightdm_conf}"
    fi
    session_changed=1
    changed=1
  done

  # Die Datei in conf.d gilt nur, wo die Hauptdatei schweigt. Sie soll aber
  # nicht das Gegenteil behaupten.
  dropin=/etc/lightdm/lightdm.conf.d/90-eventprint.conf
  if [[ -f "${dropin}" ]] && grep -q '=openbox' "${dropin}"; then
    sed -i 's/=openbox$/=labwc/' "${dropin}"
    changed=1
  fi
fi

# Die Sitzung laeuft zu diesem Zeitpunkt womoeglich schon - mit dem ALTEN
# Skript. lightdm wartet nicht auf dieses Provisioning, und die automatische
# Anmeldung ist meist schneller als Update und Build. So ist es beim Umstieg
# auf gift passiert: Die Box startete noch einmal Chromium, das keine Seite
# mehr fand, und erst ein zweiter Neustart haette es behoben. Deshalb die
# Sitzung neu starten, sobald ihr Skript getauscht wurde. Der Dienst selbst
# laeuft noch nicht (Before=eventprint.service), es geht also nichts verloren.
if [[ ${session_changed} -eq 1 ]] && systemctl is-active --quiet display-manager.service; then
  log "Kiosk-Sitzung mit dem neuen Skript neu starten"
  systemctl --no-block restart display-manager.service
fi

# ------------------------------------------------------------------- Touch ---
# Der Touch-Controller des Touch Display 2 (Goodix) hat keine Interrupt-
# Leitung und wird per Timer abgefragt, ab Werk alle 17 ms. Der Kernel zaehlt
# in 4-ms-Schritten, real kamen die Meldungen alle 24 bis 28 ms – rund 40 Hz
# bei 60 Bildern pro Sekunde. Beim Ziehen sprang der Inhalt dadurch
# abwechselnd um einen und um zwei Schritte. Mit 4 ms meldet er meist alle
# 8 ms; die Abfrage kostet kaum Rechenzeit.
touch_rule=/etc/udev/rules.d/72-eventprint-touch-poll.rules
touch_want=$(cat <<'RULE'
# Von eventprint verwaltet. Siehe scripts/provision.sh.
#
# Fragt abgefragte Goodix-Touchscreens alle 4 statt 17 ms ab. Ein Geraet mit
# Interrupt hat kein poll-Attribut und bleibt unberuehrt.
SUBSYSTEM=="input", KERNEL=="input[0-9]*", ATTR{name}=="*Goodix*", TEST=="poll", ATTR{poll}="4"
RULE
)
if [[ "$(cat "${touch_rule}" 2>/dev/null)" != "${touch_want}" ]]; then
  log "Touch-Abfrage auf 4 ms"
  printf '%s\n' "${touch_want}" >"${touch_rule}"
  udevadm control --reload-rules
  udevadm trigger --subsystem-match=input --action=change
  changed=1
fi

# ------------------------------------------------------------------ Journal ---
# Nach der ersten Veranstaltung war das Journal des Abends verloren. Raspberry
# Pi OS liefert 40-rpi-volatile-storage.conf aus und haelt die Logs im RAM, um
# die SD-Karte zu schonen. Damit stirbt beim Neustart genau die Auskunft, die
# man braucht, um einen Ausfall zu verstehen.
#
# Die 99 im Namen ist wesentlich: Drop-ins werden lexikalisch gelesen, eine 10
# vorne haette keine Wirkung gegen die 40 der Distribution.
journal_conf=/etc/systemd/journald.conf.d/99-eventprint-persistent.conf
if [[ ! -f "${journal_conf}" ]]; then
  log "Journal auf persistent stellen"
  mkdir -p /etc/systemd/journald.conf.d
  cat >"${journal_conf}" <<'CONF'
# Von eventprint gesetzt. Siehe scripts/provision.sh.
#
# Raspberry Pi OS haelt das Journal im RAM (40-rpi-volatile-storage.conf).
# Auf einem Geraet ohne Fernzugang sind Logs, die einen Neustart nicht
# ueberleben, wertlos: Nach der ersten Veranstaltung war nicht mehr
# feststellbar, warum Aufnahmen fehlten.
[Journal]
Storage=persistent
SystemMaxUse=500M
CONF
  systemctl restart systemd-journald && journalctl --flush
  changed=1
fi

# ------------------------------------------------------------------- Kamera ---
# Der Volume-Monitor von GVFS greift jede PTP-Kamera ab, sobald sie am Bus
# auftaucht, und haelt sie fest, damit man sie als Laufwerk durchblaettern
# kann. gphoto2 kommt dann nicht mehr an das Geraet heran oder verliert es
# mitten im Betrieb. Beides trifft die Fotobox an der teuersten Stelle:
# Waehrend das Tethering neu aufgebaut wird, lauscht niemand am USB, und eine
# in dieser Zeit ausgeloeste Aufnahme bleibt auf der Speicherkarte liegen.
#
# Auf einem Geraet, das den Abend ueber Fotos druckt, wird die Kamera nie als
# Laufwerk gebraucht.
for unit in gvfs-gphoto2-volume-monitor gphoto2-volume-monitor; do
  if [[ -e "/usr/libexec/${unit}" || -e "/usr/lib/gvfs/${unit}" ]]; then
    # Das Ergebnis wird in eine Variable geholt statt durch eine Pipe geprueft:
    # is-enabled liefert fuer maskierte Units einen Fehlercode, und mit
    # pipefail scheitert dann die ganze Pipeline, obwohl grep getroffen hat.
    # Die Bedingung war dadurch immer wahr und der Block lief bei jedem Start
    # erneut - genau das, was ein wiederholbares Skript nicht tun darf.
    state="$(systemctl --global is-enabled "${unit}.service" 2>/dev/null || true)"
    if [[ "${state}" != *masked* ]]; then
      log "${unit} abschalten, damit gphoto2 an die Kamera kommt"
      systemctl --global mask "${unit}.service" 2>/dev/null && changed=1
    fi
  fi
done

# Der D-Bus-Dienst startet den Monitor auch ohne systemd-Unit nach. Ihn
# stillzulegen ist der eigentlich wirksame Griff.
dbus_service=/usr/share/dbus-1/services/org.gtk.vfs.GPhoto2VolumeMonitor.service
dbus_override=/usr/local/share/dbus-1/services/org.gtk.vfs.GPhoto2VolumeMonitor.service
if [[ -f "${dbus_service}" && ! -f "${dbus_override}" ]]; then
  log "D-Bus-Aktivierung des GPhoto2-Volume-Monitors stilllegen"
  mkdir -p "$(dirname "${dbus_override}")"
  # /usr/local/share hat Vorrang vor /usr/share. Die Datei der Distribution
  # bleibt unberuehrt, ein Paketupdate ueberschreibt hier also nichts.
  cat >"${dbus_override}" <<'CONF'
[D-BUS Service]
Name=org.gtk.vfs.GPhoto2VolumeMonitor
Exec=/bin/false
CONF
  changed=1
fi

# udev: Ohne Regel gehoert der USB-Knoten der Kamera root, und der Dienst
# laeuft als eventprint. uaccess vergibt die ACL an die aktive lokale Sitzung -
# das ist der Kiosk-Nutzer, nicht der Dienst. Deshalb zusaetzlich plugdev.
udev_rule=/etc/udev/rules.d/70-eventprint-camera.rules
udev_want=$(cat <<'RULE'
# Von eventprint verwaltet. Siehe scripts/provision.sh.
#
# Gibt PTP-Kameras dem Dienstnutzer frei und haelt die Automatik des Desktops
# davon fern. Ohne die Freigabe scheitert gphoto2 mit "Could not claim the USB
# device", ohne UDISKS_IGNORE greift der Datei-Manager zu.
SUBSYSTEM=="usb", ENV{ID_GPHOTO2}=="1", MODE="0664", GROUP="plugdev", TAG+="uaccess", ENV{UDISKS_IGNORE}="1", ENV{MTP_NO_PROBE}="1"
SUBSYSTEM=="usb", ENV{ID_USB_INTERFACES}=="*:060101:*", MODE="0664", GROUP="plugdev", TAG+="uaccess", ENV{UDISKS_IGNORE}="1", ENV{MTP_NO_PROBE}="1"
RULE
)
if [[ "$(cat "${udev_rule}" 2>/dev/null)" != "${udev_want}" ]]; then
  log "udev-Regel fuer die Kamera schreiben"
  printf '%s\n' "${udev_want}" >"${udev_rule}"
  udevadm control --reload-rules
  changed=1
fi

# USB-Autosuspend: Der Kernel legt Geraete nach zwei Sekunden Leerlauf schlafen.
# Eine Kamera im Tethering ist die meiste Zeit im Leerlauf - sie wartet auf den
# Ausloeser. Schlaeft sie ein, bricht PTP, und genau das sieht aus wie "nach
# ein paar Fotos geht nichts mehr".
autosuspend_rule=/etc/udev/rules.d/71-eventprint-usb-nosuspend.rules
autosuspend_want=$(cat <<'RULE'
# Von eventprint verwaltet. Siehe scripts/provision.sh.
#
# Haelt PTP-Kameras wach. Der Kernel setzt usbcore.autosuspend auf zwei
# Sekunden; eine Kamera im Tethering wartet aber die meiste Zeit auf den
# Ausloeser und gilt damit als untaetig. Schlaeft sie ein, bricht die
# PTP-Sitzung.
SUBSYSTEM=="usb", ENV{ID_GPHOTO2}=="1", TEST=="power/control", ATTR{power/control}="on"
SUBSYSTEM=="usb", ENV{ID_USB_INTERFACES}=="*:060101:*", TEST=="power/control", ATTR{power/control}="on"
RULE
)
if [[ "$(cat "${autosuspend_rule}" 2>/dev/null)" != "${autosuspend_want}" ]]; then
  log "USB-Autosuspend fuer Kameras abschalten"
  printf '%s\n' "${autosuspend_want}" >"${autosuspend_rule}"
  udevadm control --reload-rules
  changed=1
fi

if [[ ${changed} -eq 0 ]]; then
  log "nichts zu tun, die Box stimmt bereits"
else
  log "fertig"
fi

exit 0
