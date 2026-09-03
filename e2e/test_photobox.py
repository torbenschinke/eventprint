"""End-to-End-Tests der Fotobox.

Hier steht ausschließlich, was **nur** ein Browser beweisen kann: dass die
Oberfläche überhaupt erscheint, dass die Seiten erreichbar sind und dass der
Ablauf eines Abends im Zusammenspiel funktioniert.

Nago ist eine Single-Page-Anwendung, die ihren gesamten Zustand über eine
WebSocket-Verbindung bezieht. Ein HTTP-Abruf sagt darüber nichts. Genau dafür
sind diese Tests da – und für nichts sonst.

Alles, was eine Aussage über Go-Werte ist, gehört in `go test`: Rechte an
Rollen, Vorbelegungen aus der Umgebung, der Zustand des Druckers, der Weg vom
Import bis zum fertigen Auftrag. Das läuft dort in Millisekunden statt hier in
einem Anwendungsstart je Test. Die Zuordnung steht in der README unter
„Tests“.
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

    # Die Tests laufen ohne Drucker. Die Anwendung muss den Testbetrieb
    # benennen und darf keinen Ausdruck versprechen – ein Gast, der vergeblich
    # am Drucker wartet, ist schlimmer als eine ehrliche Absage.
    expect(page.get_by_text("Testmodus").first).to_be_visible(timeout=30_000)
    expect(page.get_by_text("Wird gedruckt")).to_have_count(0)


def login_as_operator(page: Page, base_url: str) -> None:
    """Meldet den Bootstrap-Administrator über das Formular an.

    Die Anmeldung ist auf dem Touchscreen bewusst nicht mehr im Menü: Dort gibt
    es keine Tastatur, der Weg hinein ist die PIN. Vom Smartphone aus bleibt
    das Formular unter /account/login erreichbar, und genau das prüft dieser
    Weg.
    """
    page.goto(f"{base_url}/account/login")

    page.locator("input").nth(0).fill(ADMIN_MAIL)
    page.locator("input").nth(1).fill(ADMIN_PASSWORD)
    page.get_by_role("button", name="Anmelden").last.click()

    # Nach erfolgreicher Anmeldung erscheint die Menüleiste der Betreuung.
    expect(page.get_by_text("Druckstatus").first).to_be_visible(timeout=30_000)


def test_empty_booth_shows_nothing_but_the_booth(page: Page, server: str):
    """Die leere Fotobox: Startbildschirm, sonst nichts.

    Der wichtigste Test überhaupt, denn er beantwortet die Frage, ob die
    Oberfläche beim Aufbau erscheint. Alles Weitere setzt das voraus.

    Dazu die Gegenprobe, dass ein Gast nichts weiter sieht: Der Touchscreen
    misst 1024x600 und steht den Abend über zwischen Gästen. Menüleiste,
    Anmeldung und Fußzeile wären dort nur Angebote, sich zu verlaufen.
    """
    page.goto(server)

    expect(page.get_by_text(EVENT_TITLE)).to_be_visible()
    expect(page.get_by_text("Noch keine Fotos")).to_be_visible()

    # Der QR-Code muss die von außen erreichbare Upload-Adresse enthalten,
    # sonst läuft der Gast ins Leere.
    expect(page.get_by_text(f"{server}/upload")).to_be_visible()

    # Kein Rahmen: keine Menüleiste, keine Fußzeile.
    #
    # Auf das Element selbst prüfen und nicht auf die Beschriftungen. Genau
    # dieser Test war zuvor grün, während auf dem Gerät ein leerer Balken
    # stand: Die Einträge waren weg, die Leiste nicht. Ein Test, der nur die
    # Texte zählt, sieht das nicht.
    expect(page.locator("nav")).to_have_count(0)

    for label in ["Alle Fotos", "Druckstatus", "Hochladen", "Abmelden", "Anmelden"]:
        expect(page.get_by_text(label, exact=True)).to_have_count(0)

    # Die Fußzeile mit Impressum und Nutzungsbedingungen kostet auf 600 Punkten
    # Höhe nur Platz und richtet sich an niemanden, der hier steht.
    expect(page.get_by_text("Impressum")).to_have_count(0)

    # Die Seiten selbst bleiben erreichbar, nur eben nicht über ein Menü.
    page.goto(f"{server}/print/status")
    expect(page.get_by_text("Es ist kein Drucker eingerichtet")).to_be_visible(timeout=30_000)
    expect(page.get_by_text("Zum Einrichten als Betreuer anmelden.")).to_be_visible()


def test_guest_upload_print_and_reprint(page: Page, server: str):
    """Der Abend in einem Test: hochladen, drucken, nachdrucken.

    Bewusst ein einziger, durchgehender Ablauf statt vier getrennter Tests:
    Jeder Test startet die Anwendung neu, und die Zwischenschritte sind
    ohnehin nur als Kette sinnvoll.
    """
    upload_photo(page, server)

    page.get_by_role("button", name="Drucken").first.click()
    choose_template_and_print(page, "Polaroid")

    # Der Auftrag muss im Druckstatus auftauchen und dort auch fertig werden.
    page.goto(f"{server}/print/status")
    expect(page.get_by_text("Polaroid")).to_be_visible(timeout=30_000)
    expect(page.get_by_text("Fertig")).to_be_visible(timeout=60_000)

    # Das Bild gehört in die Historie – unabhängig davon, wer es hochlud.
    page.goto(f"{server}/gallery")
    expect(page.get_by_text("Gast-Upload")).to_be_visible(timeout=30_000)
    expect(photo_tiles(page)).not_to_have_count(0)

    # Auf dem Startbildschirm erscheint es ohne Zutun: Die Seite zeichnet sich
    # zyklisch neu, und genau das lässt sich nur im Browser zeigen.
    page.goto(server)
    expect(page.get_by_text("Noch keine Fotos")).to_have_count(0, timeout=30_000)
    expect(photo_tiles(page)).not_to_have_count(0)

    # Eine Kachel antippen öffnet die Layout-Auswahl für den Nachdruck.
    photo_tiles(page).first.click()
    choose_template_and_print(page, "Passepartout")

    page.goto(f"{server}/print/status")
    expect(page.get_by_text("Passepartout")).to_be_visible(timeout=30_000)


def tap_qr_code(page: Page, times: int) -> None:
    """Tippt den QR-Code an, um die verborgene Einrichtung zu öffnen."""
    qr = page.get_by_role("img", name="QR-Code zum Hochladen eigener Fotos")
    for _ in range(times):
        qr.click()


def enter_pin(page: Page, pin: str) -> None:
    """Tippt eine PIN auf dem Tastenfeld der Fotobox."""
    for digit in pin:
        page.get_by_text(digit, exact=True).last.click()


def test_pin_unlocks_the_configuration_on_a_factory_new_box(page: Page, server: str):
    """Fünf Berührungen des QR-Codes, PIN vergeben, Betreuer sein.

    Das ist die eine Kette, die kein Go-Test abdecken kann: die verborgene
    Geste, das Tastenfeld und die Frage, ob die Freischaltung einen
    Seitenwechsel überlebt. Die Regeln dahinter – Sperre, Ablauf, Form der PIN –
    prüfen die Go-Tests in `app/photobox/cfg/`.

    Der Testlauf startet mit leerem Datenverzeichnis, die Fotobox ist also
    fabrikneu und hat noch keine PIN.
    """
    pin = "481902"

    page.goto(server)

    # Ohne die Geste ist von der Einrichtung nichts zu sehen.
    expect(page.get_by_text("PIN festlegen")).to_have_count(0)

    # Vier Berührungen dürfen noch nichts öffnen.
    tap_qr_code(page, 4)
    expect(page.get_by_text("PIN festlegen")).to_have_count(0)

    # Die fünfte öffnet das Tastenfeld im Vergabemodus.
    tap_qr_code(page, 1)
    expect(page.get_by_text("PIN festlegen")).to_be_visible(timeout=30_000)
    expect(page.get_by_text("Noch keine PIN vergeben.")).to_be_visible()

    # Zweimal dieselbe PIN, sonst wäre ein Vertipper nicht zu bemerken.
    enter_pin(page, pin)
    expect(page.get_by_text("Zur Sicherheit noch einmal dieselbe PIN eingeben.")).to_be_visible(timeout=30_000)
    enter_pin(page, pin)

    # Wer die PIN vergeben hat, ist damit Betreuer.
    expect(page.get_by_text("PIN festlegen")).to_have_count(0, timeout=30_000)

    # Jetzt erscheint der Rahmen mit der Menüleiste der Betreuung.
    expect(page.locator("nav")).not_to_have_count(0, timeout=30_000)

    for label in ["Alle Fotos", "Druckstatus", "Abmelden"]:
        expect(page.get_by_text(label, exact=True).first).to_be_visible(timeout=30_000)

    # Und die Einrichtung ist offen, auch nach einem echten Seitenwechsel.
    page.goto(f"{server}/print/status")
    expect(page.get_by_role("button", name="Drucker einrichten")).to_be_visible(timeout=30_000)

    # Der eigentliche Nachweis: Die PIN meldet in ein echtes Konto an.
    #
    # Nagos Einstellungsseiten prüfen ihre eigenen Berechtigungen
    # (nago.settings.global.*), nicht die dieser Anwendung. Mit einem
    # selbstgebauten Subject öffnete sich die Seite zwar, aber das Speichern
    # scheiterte an einer Rechteverletzung – die Fotobox liess sich nicht
    # einrichten, obwohl die PIN stimmte.
    neuer_titel = "Sommerfest 2026"

    page.goto(server)
    page.get_by_role("button", name="Öffentliche Adresse setzen").click()
    expect(page.get_by_label("Titel der Veranstaltung")).to_be_visible(timeout=30_000)

    page.get_by_label("Titel der Veranstaltung").fill(neuer_titel)
    page.get_by_role("button", name="Speichern").click()

    expect(page.get_by_text("Zugriff verweigert")).to_have_count(0, timeout=30_000)

    # Gespeichert ist erst, was auch ankommt.
    page.goto(server)
    expect(page.get_by_text(neuer_titel)).to_be_visible(timeout=30_000)

    # Abmelden nimmt die Rechte wieder.
    page.goto(server)
    page.get_by_text("Abmelden", exact=True).first.click()
    expect(page.get_by_text("Druckstatus", exact=True)).to_have_count(0, timeout=30_000)


def test_operator_keeps_access_after_login(page: Page, server: str):
    """Anmelden darf den Zugriff auf die Fotobox nicht entziehen."""
    login_as_operator(page, server)

    page.goto(f"{server}/gallery")
    expect(page.get_by_text("Noch keine Fotos")).to_be_visible()

    page.goto(f"{server}/print/status")
    expect(page.get_by_text("Druckstatus")).to_be_visible()
    # Nur der Betreuer bekommt die Einrichtung angeboten.
    expect(page.get_by_role("button", name="Drucker einrichten")).to_be_visible()


def test_public_url_setting_changes_qr_code(page: Page, server: str):
    """Die öffentliche Adresse muss von Hand setzbar sein.

    Hinter einem Reverse Proxy leitet Nago sonst einen Namen aus der ersten
    Verbindung ab, der für Gäste unerreichbar ist – der QR-Code wäre wertlos.

    Die Bildung der Adresse selbst prüft TestPublicURLFor in Go; hier geht es
    um den Weg durch das Einstellungsformular bis in den QR-Code.
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
