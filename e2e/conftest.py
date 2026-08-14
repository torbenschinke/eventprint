"""Gemeinsame Fixtures für die End-to-End-Tests der Fotobox.

Die Nago-Oberfläche ist eine Single-Page-Anwendung, die ihren gesamten
Zustand über eine WebSocket-Verbindung (/wire) bezieht. Ein einfacher
HTTP-Abruf sagt daher nichts über die Funktion aus – die Tests steuern
deshalb einen echten Browser.
"""

import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

import pytest
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parent.parent

# Beispielbild, das ein Gast hochlädt. Bewusst ein echtes Kamerabild im
# 16:9-Format, damit der Zuschnitt auf 3:2 tatsächlich stattfindet.
SAMPLE_IMAGE = ROOT / "DSC02301.jpg"

EVENT_TITLE = "Hochzeit von Anna & Ben"

# Zugangsdaten des Bootstrap-Administrators, siehe cmd/photobox/main.go.
ADMIN_MAIL = "admin@localhost"
ADMIN_PASSWORD = "%6UbRsCuM8N$auy"

# Nago verzichtet bewusst darauf, die SPA für Bots zu mounten: Der Bundle
# prüft den User-Agent gegen eine Crawler-Liste und ruft mount("#app") gar
# nicht erst auf. Der Standard-User-Agent von Playwright enthält
# "HeadlessChrome" und fällt in genau dieses Raster – ohne einen realistischen
# User-Agent bliebe die Seite dauerhaft leer.
USER_AGENT = (
    "Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 "
    "(KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
)


def _free_port() -> int:
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


@pytest.fixture(scope="session")
def binary(tmp_path_factory) -> Path:
    """Übersetzt die Anwendung einmal pro Testlauf."""
    out = tmp_path_factory.mktemp("bin") / "photobox"
    subprocess.run(
        ["go", "build", "-o", str(out), "./cmd/photobox"],
        cwd=ROOT,
        check=True,
    )
    return out


def _start(binary, extra_env: dict[str, str] | None = None):
    """Startet die Fotobox in einem eigenen, leeren Datenverzeichnis.

    Die Fixture ist bewusst je Test neu und nicht für die ganze Sitzung:
    Fotos und Druckaufträge werden dauerhaft gespeichert, ein geteilter Server
    würde die Tests voneinander abhängig machen. Insbesondere die Prüfungen
    auf den leeren Anfangszustand wären dann von der Ausführungsreihenfolge
    abhängig.

    Ohne EVENTPRINT_PRINTER läuft der Druck im Testmodus: Die Aufträge
    durchlaufen dieselbe Zustandsmaschine, das gerenderte Bild wird aber
    verworfen statt an CUPS übergeben.
    """
    home = tempfile.mkdtemp(prefix="photobox-e2e-")
    port = _free_port()

    env = {
        **os.environ,
        "HOME": home,
        "PORT": str(port),
        "HOST": "127.0.0.1",
        "EVENTPRINT_TITLE": EVENT_TITLE,
        # kein EVENTPRINT_PRINTER -> Testmodus
        "NAGO_COOKIES_INSECURE": "true",
        **(extra_env or {}),
    }

    log = open(os.path.join(home, "photobox.log"), "w+")
    proc = subprocess.Popen([str(binary)], env=env, stdout=log, stderr=subprocess.STDOUT)

    base_url = f"http://127.0.0.1:{port}"

    # auf den Listener warten
    deadline = time.time() + 60
    while time.time() < deadline:
        if proc.poll() is not None:
            log.seek(0)
            raise RuntimeError("Fotobox beendet sich sofort:\n" + log.read())
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=0.5):
                break
        except OSError:
            time.sleep(0.25)
    else:
        raise RuntimeError("Fotobox ist nicht gestartet")

    yield base_url

    proc.terminate()
    try:
        proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        proc.kill()

    log.seek(0)
    output = log.read()
    log.close()
    shutil.rmtree(home, ignore_errors=True)

    # Das Protokoll nur im Fehlerfall ausgeben, sonst ertrinkt der Bericht
    # darin. pytest zeigt es bei bestandenen Tests ohnehin nicht an.
    sys.stderr.write(output)


@pytest.fixture
def server(binary):
    """Eine Fotobox ohne konfigurierten Drucker, also im Testmodus."""
    yield from _start(binary)


@pytest.fixture
def server_with_printer(binary):
    """Eine Fotobox mit vorbelegter Warteschlange.

    Der Name ist frei erfunden: Geprüft wird, dass die Vorbelegung aus der
    Umgebung in den Einstellungen ankommt und die Oberfläche das Ziel meldet.
    Ob die Warteschlange existiert, ist dafür unerheblich – so läuft der Test
    auch auf einem Rechner ohne CUPS.
    """
    yield from _start(binary, {"EVENTPRINT_PRINTER": "CZ01-e2e"})


@pytest.fixture(scope="session")
def browser():
    with sync_playwright() as pw:
        browser = pw.chromium.launch()
        yield browser
        browser.close()


def _new_page(browser):
    context = browser.new_context(
        viewport={"width": 1600, "height": 1000},
        user_agent=USER_AGENT,
    )
    page = context.new_page()
    page.set_default_timeout(30_000)
    return context, page


@pytest.fixture
def page(browser, server):
    """Eine frische Sitzung je Test, damit sich Zustände nicht vermischen."""
    context, page = _new_page(browser)
    yield page
    context.close()


@pytest.fixture
def page_with_printer(browser, server_with_printer):
    """Wie [page], nur gegen die Fotobox mit vorbelegter Warteschlange."""
    context, page = _new_page(browser)
    yield page
    context.close()
