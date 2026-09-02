"""End-to-End-Tests der Fotobox.

Die Tests bilden den tatsächlichen Ablauf einer Veranstaltung nach: Ein Gast
scannt den QR-Code, lädt ein Bild hoch, wählt ein Layout und druckt. Danach
muss das Bild in der Historie stehen und der Auftrag im Druckstatus sichtbar
sein.
"""

import re

from playwright.sync_api import Locator, Page, expect

from conftest import ADMIN_MAIL, ADMIN_PASSWORD, EVENT_TITLE, SAMPLE_IMAGE


def photo_tiles(page: Page) -> Locator:
    """Die Foto-Kacheln.

    Die Bezeichnung stammt aus der AccessibilityLabel der Kachel und ist der
    ursprüngliche Dateiname. Das grenzt sie sauber gegen den QR-Code ab, der
    ebenfalls als Bild gerendert wird.
    """
    return page.get_by_role("img", name=SAMPLE_IMAGE.name)


def upload_photo(page: Page, base_url: str) -> None:
    """Lädt das Beispielbild über die öffentliche Upload-Seite hoch."""
    page.goto(f"{base_url}/upload")
    expect(page.get_by_text("Dein Foto drucken")).to_be_visible()

    # ImportFiles öffnet den Dateidialog des Browsers.
    with page.expect_file_chooser() as fc:
        page.get_by_role("button", name="Foto auswählen").click()

    fc.value.set_files(str(SAMPLE_IMAGE))

    expect(page.get_by_text("Hochgeladen – jetzt drucken:")).to_be_visible(timeout=60_000)


def choose_template_and_print(page: Page, template: str) -> None:
    """Wählt im Dialog ein Layout und bestätigt den Druck."""
    expect(page.get_by_text("Wie soll gedruckt werden?")).to_be_visible()

    page.get_by_text(template, exact=True).click()
    page.get_by_role("button", name=re.compile("Bestätigen|Confirm|OK", re.I)).click()

    # Die Tests laufen ohne Drucker, deshalb bestätigt die Anwendung den
    # Testbetrieb statt einen Ausdruck. Nago rendert das Banner zusätzlich in
    # einem Overlay, deshalb genügt der erste Treffer.
    expect(page.get_by_text("Testmodus").first).to_be_visible(timeout=30_000)


def test_booth_screen_without_photos(page: Page, server: str):
    """Der Startbildschirm begrüßt mit Titel, QR-Code und leerer Historie."""
    page.goto(server)

    expect(page.get_by_text(EVENT_TITLE)).to_be_visible()
    expect(page.get_by_text("Noch keine Fotos")).to_be_visible()

    # Der QR-Code muss die von außen erreichbare Upload-Adresse enthalten,
    # sonst läuft der Gast ins Leere.
    expect(page.get_by_text(f"{server}/upload")).to_be_visible()


def test_print_status_warns_about_missing_printer(page: Page, server: str):
    """Ohne konfigurierten Drucker muss die Fotobox das deutlich sagen.

    Der Testlauf startet die Anwendung ohne EVENTPRINT_PRINTER; wer sie so
    aufbaut, soll den Grund sofort sehen statt den Fehler beim Drucker zu
    suchen.
    """
    page.goto(f"{server}/print/status")

    expect(page.get_by_text("Es ist kein Drucker eingerichtet")).to_be_visible()
    # Gäste sollen die Einrichtung nicht angeboten bekommen.
    expect(page.get_by_text("Zum Einrichten als Betreuer anmelden.")).to_be_visible()


def test_print_in_test_mode_does_not_claim_to_print(page: Page, server: str):
    """Im Testbetrieb darf die Bestätigung keinen Ausdruck versprechen."""
    upload_photo(page, server)

    page.get_by_role("button", name="Drucken").first.click()
    expect(page.get_by_text("Wie soll gedruckt werden?")).to_be_visible()
    page.get_by_role("button", name=re.compile("Bestätigen|Confirm|OK", re.I)).click()

    expect(page.get_by_text("Testmodus").first).to_be_visible(timeout=30_000)
    expect(page.get_by_text("Wird gedruckt")).to_have_count(0)


def test_scaffold_menu_links_all_pages(page: Page, server: str):
    """Alle Seiten sind ohne Anmeldung über das Menü erreichbar."""
    page.goto(server)

    # Die Menüeinträge des Scaffolds sind keine <a>-Elemente, sondern
    # klickbare Container – daher über den Text ansteuern.
    for label, marker in [
        ("Alle Fotos", "Noch keine Fotos"),
        ("Druckstatus", "Keine Druckaufträge"),
    ]:
        page.get_by_text(label, exact=True).first.click()
        expect(page.get_by_text(marker).first).to_be_visible(timeout=30_000)


def test_guest_upload_and_print(page: Page, server: str):
    """Der vollständige Gast-Ablauf: hochladen, Layout wählen, drucken."""
    upload_photo(page, server)

    page.get_by_role("button", name="Drucken").first.click()
    choose_template_and_print(page, "Polaroid")

    # Der Auftrag muss im Druckstatus auftauchen und dort auch fertig werden.
    page.goto(f"{server}/print/status")
    expect(page.get_by_text("Polaroid")).to_be_visible(timeout=30_000)
    expect(page.get_by_text("Fertig")).to_be_visible(timeout=60_000)

    # Und das Bild gehört in die Historie – unabhängig davon, wer es hochlud.
    page.goto(f"{server}/gallery")
    expect(page.get_by_text("Gast-Upload")).to_be_visible(timeout=30_000)
    expect(photo_tiles(page)).not_to_have_count(0)


def test_uploaded_photo_appears_on_booth_screen(page: Page, server: str):
    """Ein Gast-Upload erscheint auf dem Fotobox-Display der Bedienung."""
    upload_photo(page, server)

    page.goto(server)

    # Der Startbildschirm zeichnet sich zyklisch neu; das Bild muss ohne
    # Zutun erscheinen.
    expect(page.get_by_text("Noch keine Fotos")).to_have_count(0, timeout=30_000)
    expect(photo_tiles(page)).not_to_have_count(0)


def test_reprint_from_booth_screen(page: Page, server: str):
    """Ein Bild auf dem Startbildschirm lässt sich erneut drucken."""
    upload_photo(page, server)

    page.goto(server)
    expect(page.get_by_text("Noch keine Fotos")).to_have_count(0, timeout=30_000)

    # Kachel antippen öffnet die Layout-Auswahl.
    photo_tiles(page).first.click()
    choose_template_and_print(page, "Passepartout")

    page.goto(f"{server}/print/status")
    expect(page.get_by_text("Passepartout")).to_be_visible(timeout=30_000)


def login_as_operator(page: Page, base_url: str) -> None:
    """Meldet den Bootstrap-Administrator an.

    Er trägt die Rolle "Fotobox-Betreuer" und darf damit einrichten. Ohne
    diese Rolle zeigte die Anwendung nach dem Anmelden auf jeder Seite
    "Zugriff verweigert", weil Nago dem Bootstrap-Konto bewusst nur
    nago.*-Berechtigungen gibt.
    """
    page.goto(base_url)
    page.get_by_text("Anmelden").first.click()

    page.locator("input").nth(0).fill(ADMIN_MAIL)
    page.locator("input").nth(1).fill(ADMIN_PASSWORD)
    page.get_by_role("button", name="Anmelden").last.click()

    # Nach erfolgreicher Anmeldung verschwindet der Anmelden-Eintrag.
    expect(page.get_by_text("Anmelden")).to_have_count(0, timeout=30_000)


def test_operator_keeps_access_after_login(page: Page, server: str):
    """Anmelden darf den Zugriff auf die Fotobox nicht entziehen."""
    login_as_operator(page, server)

    page.goto(f"{server}/gallery")
    expect(page.get_by_text("Noch keine Fotos")).to_be_visible()

    page.goto(f"{server}/print/status")
    expect(page.get_by_text("Druckstatus")).to_be_visible()
    # Nur der Betreuer bekommt die Einrichtung angeboten.
    expect(page.get_by_role("button", name="Drucker einrichten")).to_be_visible()


def test_printer_from_environment_is_used(page_with_printer: Page, server_with_printer: str):
    """Die Vorbelegung aus der Umgebung landet in den Einstellungen."""
    page_with_printer.goto(f"{server_with_printer}/print/status")

    expect(page_with_printer.get_by_text("Drucker: CZ01-e2e")).to_be_visible()
    expect(page_with_printer.get_by_text("Es ist kein Drucker eingerichtet")).to_have_count(0)


def test_print_status_reports_unknown_queue(page_with_printer: Page, server_with_printer: str):
    """Eine nicht eingerichtete Warteschlange muss auffallen.

    Die Fotobox ist auf CZ01-e2e eingestellt, das es in CUPS nicht gibt. Ohne
    diese Meldung liefen alle Aufträge ins Leere und niemand wüsste warum –
    genau der Fall, der uns mehrere Druckversuche gekostet hat.
    """
    page_with_printer.goto(f"{server_with_printer}/print/status")

    expect(page_with_printer.get_by_text("Der Drucker ist nicht bereit")).to_be_visible()
    expect(
        page_with_printer.get_by_text("Die Warteschlange CZ01-e2e ist in CUPS nicht eingerichtet.")
    ).to_be_visible()


def test_public_url_setting_changes_qr_code(page: Page, server: str):
    """Die öffentliche Adresse muss von Hand setzbar sein.

    Hinter einem Reverse Proxy leitet Nago sonst einen Namen aus der ersten
    Verbindung ab, der für Gäste unerreichbar ist – der QR-Code wäre wertlos.
    """
    public_url = "https://fotobox.example.de"

    login_as_operator(page, server)

    # Der Hinweis erscheint, weil im QR-Code eine lokale Adresse steht.
    expect(page.get_by_text("Diese Adresse erreicht nur dieser Rechner.")).to_be_visible()
    page.get_by_role("button", name="Öffentliche Adresse setzen").click()

    expect(page.get_by_text("Öffentliche Adresse")).to_be_visible(timeout=30_000)
    page.get_by_label("Öffentliche Adresse").fill(public_url)
    page.get_by_role("button", name="Speichern").click()

    page.goto(server)
    expect(page.get_by_text(f"{public_url}/upload")).to_be_visible(timeout=30_000)
    expect(page.get_by_text("Diese Adresse erreicht nur dieser Rechner.")).to_have_count(0)
