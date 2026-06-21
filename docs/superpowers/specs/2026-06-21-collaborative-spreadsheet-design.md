# Design: Kollaborative Tabellenkalkulation (Spreadsheet) für etherpad-go

**Datum:** 2026-06-21
**Status:** Entwurf zur Umsetzung freigegeben
**Scope dieser Spec:** Erster Dokumenttyp `sheet` (Excel-artig) inklusive des gemeinsamen Dokumenttyp-Fundaments. Der Präsentationstyp (PowerPoint-artig) ist ein späteres, eigenes Teilprojekt, das auf diesem Fundament aufsetzt.

---

## 1. Ziel & Kontext

etherpad-go ist heute ein reiner Text-Editor: Pads basieren auf Changeset-OT, `AText` (attributierter Text), Socket.IO für Echtzeit-Kollaboration und einem Hook-basierten Plugin-System.

Ziel ist, neben Text-Pads auch **kollaborative Tabellen** zu unterstützen — mehrere Nutzer bearbeiten dieselbe Tabelle gleichzeitig, mit Formeln und xlsx-Kompatibilität.

### Festgelegter Scope (v1)

| Dimension | Entscheidung |
|-----------|--------------|
| Kollaboration | Echte Echtzeit-Kollaboration (mehrere Nutzer gleichzeitig) |
| Formeln | Volle Formel-Engine (~400 Funktionen) via HyperFormula, **clientseitig** |
| xlsx | Import **und** Export nötig |
| Architektur | Eigene Kollaborationsschicht auf bewährten Bausteinen, maximale Wiederverwendung der etherpad-go-Infrastruktur |
| Erster Typ | Spreadsheet (als Vorlage für das generische Dokument-Fundament) |

### Bewusst NICHT in v1 (spätere Teilprojekte)

- Serverseitige Formel-Neuberechnung (Go-Engine)
- Präsentationstyp (PowerPoint)
- Charts, Pivot-Tabellen, eingebettete Bilder, Makros, bedingte Formatierung, Data Validation
- Zeichengenaues Merge *innerhalb* einer einzelnen Zelle

---

## 2. Dokument-Modell & Integration

### Dokumenttyp-Konzept

Heute ist „Pad" implizit gleich „Text-Dokument". Wir führen ein explizites Typ-Feld ein:

- **`DocumentType`** am Pad/Dokument: `text` (Bestand, Default) | `sheet` (neu).
- Bestehende Pads bleiben implizit `text` → **keine Migration nötig, abwärtskompatibel**.

### Wiederverwendete gemeinsame Infrastruktur

- Pad-ID-Format inkl. Gruppen-Präfix (`g.XXXXXXXXXXXXXXXX$name`)
- Auth / `preAuthorize`-Hooks, Readonly-IDs, Gruppen
- Session- und Author-Verwaltung
- Socket.IO-Transport
- **Per-Pad-Goroutine-Serialisierung** → liefert die totale Op-Reihenfolge gratis

### Getrenntes Inhaltsmodell

Statt `AText` + Changesets bekommt ein `sheet`-Dokument ein eigenes Workbook-Modell und einen eigenen Op-Stream (eigene Tabellen, eigener Message-Handler). **Der Text-Pfad bleibt unangetastet.**

### Routing / UX

- `/p/:pad` → Text-Editor (wie bisher)
- `/s/:pad` → Spreadsheet-Editor
- Welcome-/Erstell-Seite: Typ-Auswahl („Neues Pad" vs. „Neue Tabelle"). Beim Anlegen wird `DocumentType` gesetzt; danach entscheidet der Typ über Route, Editor-Frontend und Message-Handler.

### Leitgedanke

Ein gemeinsamer „Dokument"-Rahmen (Identität, Zugriff, Transport, Präsenz), aber **pro Typ ein eigenes Inhalts- und Op-Modell**.

---

## 3. Datenmodell der Tabelle & Op-Format

### Workbook-Struktur

```
Workbook (= ein "sheet"-Dokument)
 └─ Sheets[]            (Tabellenblätter, geordnet, benannt)
     ├─ Cells           (sparse, nur belegte Zellen gespeichert)
     ├─ RowProps[]      (Höhe, ausgeblendet)
     ├─ ColProps[]      (Breite, ausgeblendet)
     └─ MergedRanges[]  (verbundene Zellen)
```

### Zelle

Adressiert über `(sheetId, row, col)` — 0-basiert, intern numerisch (kein „A1"-Parsing im Speicher).

```
Cell {
  kind:    "value" | "formula"
  raw:     string        // Roheingabe ("42", "=SUM(A1:A10)", "Hallo")
  value:   string|number|bool|null   // berechnetes Ergebnis (von HyperFormula)
  type:    number|text|bool|date|error   // erkannter Datentyp
  styleId: int           // Verweis in einen Style-Pool
}
```

**Style-Pool** (wiederverwendetes Pattern des AText-Attribut-Pools): Formatierungen (Zahlenformat, Schrift, Farben, Rahmen, Ausrichtung) werden dedupliziert pro Workbook gehalten; Zellen referenzieren nur eine `styleId`. Hält Speicher und Ops klein.

### Op-Format (kollaboratives Herzstück)

Kompakte, serialisierbare Operation mit Revisionsnummer (zellbasiert, nicht text-changeset-basiert):

```
Op {
  rev:   int             // Basis-Revision des Clients
  type:  "setCell" | "setStyle" | "clearRange" | "setRange" |
         "insertRows" | "deleteRows" | "insertCols" | "deleteCols" |
         "addSheet" | "removeSheet" | "renameSheet" |
         "setRowProp" | "setColProp" | "merge" | "unmerge"
  sheet: sheetId
  payload: { ... }       // typ-abhängig, z.B. {row, col, raw, styleId}
}
```

### Konfliktauflösung — bewusst minimal

- **Server gibt die totale Reihenfolge vor** (vorhandene Per-Pad-Goroutine). Jeder Op erhält eine fortlaufende `rev`.
- **Zell-Ops kommutieren:** unterschiedliche Zellen → kein Konflikt. Gleiche Zelle → serverseitig späterer Op gewinnt (Last-Writer-Wins auf Zellebene). Kein zeichengenaues Merge innerhalb einer Zelle in v1 — eine Zelle ist atomar.
- **Struktur-Ops** (`insertRows`/`deleteRows`/`insertCols`/`deleteCols`) brauchen **Index-Transformation:** Kommt ein Client-Op mit veralteter `rev`, transformiert der Server die Indizes gegen die zwischenzeitlich angewandten Struktur-Ops. Begrenzte, gut testbare OT-Logik — nur Index-Verschiebung, nicht der volle Text-OT-Apparat.
- Clients wenden Ops **optimistisch lokal** an und reconcilen beim Server-Ack (gleiches Muster wie der bestehende `collab_client`).

### Persistenz (über die `DataStore`-Abstraktion: SQLite/Postgres/MySQL)

- `sheet` — Workbook-Metadaten (Sheets-Liste, Head-Rev, Style-Pool als JSON)
- `sheet_cell` — belegte Zellen `(doc, sheet, row, col, raw, value, type, styleId)`, sparse
- `sheet_op` — Op-Log pro Dokument für Reconnect/History/Timeslider (analog `pad_revision`); periodische Zustands-Snapshots für schnelles Laden (analog zu Key-Revisions alle 100)

### Leitgedanke

Zellen sind **atomare Einheiten mit totaler Server-Ordnung** — das macht Kollaboration drastisch einfacher als Text-OT und nutzt trotzdem genau die vorhandene Infrastruktur.

---

## 4. Formel-Engine & Berechnungs-Fluss

### Engine: HyperFormula (Client) als primäre Berechnung

Headless Formel-Engine (~400 Funktionen, Abhängigkeitsgraph, Array-Formeln, A1- & R1C1-Notation). Läuft im Browser, hält pro geöffnetem Dokument eine In-Memory-Instanz parallel zum Grid.

> **Lizenz-Vorbehalt (offene Frage 1):** HyperFormula ist GPLv3 **oder** kommerziell. Das muss bewusst akzeptiert oder vorab geklärt werden — es beeinflusst die Lizenz des Gesamtprojekts. Falls GPLv3 nicht passt, ist dies die Stelle für eine MIT-Alternative.

### Berechnungs-Fluss

```
Nutzer tippt "=SUM(A1:A10)" in B2
   │
   ▼
1. Client erzeugt Op{setCell, raw:"=SUM(A1:A10)"}  → optimistisch lokal
2. Client füttert raw in lokale HyperFormula-Instanz
3. HyperFormula liefert berechneten value + betroffene abhängige Zellen
4. Grid rendert value; Op geht an den Server
   │
   ▼ Server
5. Per-Pad-Serialisierung vergibt rev, persistiert raw + (Snapshot-)value
6. Broadcast des Op an alle anderen Clients
   │
   ▼ andere Clients
7. wenden Op an, füttern raw in ihre HyperFormula-Instanz → identische Neuberechnung
```

### Quelle der Wahrheit

- **`raw` (Formel/Eingabe) ist die Quelle der Wahrheit** und wird kollaborativ synchronisiert. `value` ist abgeleitet.
- Jeder Client berechnet `value` deterministisch selbst aus `raw` → keine Werte-Konflikte. Der Server speichert den zuletzt gemeldeten `value` nur als **Cache** (schnelles Laden, Suche, xlsx-Export ohne JS-Engine auf dem Server).
- **Determinismus-Vorbehalt:** Volatile Funktionen (`NOW()`, `RAND()`) liefern pro Client andere Werte. v1-Regel: der `value` des **schreibenden** Clients wird mitgesendet und als Snapshot übernommen; Neuberechnung volatiler Zellen passiert nur bei tatsächlicher Bearbeitung, nicht spontan pro Client. Vermeidet Flackern.

### Serverseitige Berechnung — NICHT in v1

Eine zweite, serverseitige Engine (Go) wäre nötig, um Werte ohne offenen Browser aktuell zu halten und per API auf berechnete Werte zuzugreifen. Eigenes Teilprojekt. Der `value`-Cache aus Schritt 5 deckt die wichtigsten Fälle (xlsx-Export, Anzeige des letzten Stands) ab.

### Leitgedanke

`raw` wird synchronisiert, `value` wird lokal & deterministisch abgeleitet — dadurch bleibt das kollaborative Modell aus Abschnitt 3 unberührt; die Formel-Engine ist „nur" eine lokale Ableitungsschicht.

---

## 5. Frontend & Grid-UI

### Einordnung

- Neuer Vite-**Entry-Point `sheet.entry.ts`**, ausgeliefert über `/s/:pad`. Das Text-Editor-Bundle bleibt komplett getrennt.
- Wiederverwendet werden typ-unabhängige Module: `socketio.ts` (Transport), `pad_connectionstatus.ts`, `pad_userlist.ts` (Präsenz/Cursor), Auth/Session-Bootstrap.

### Grid-Komponente — Build vs. Buy

Performantes Grid (virtualisiertes Rendering für 100.000+ Zellen, Auswahl, Inline-Edit, Resize, Frozen Rows) baut man nicht mit DOM-`<td>`s.

- **Empfehlung:** Canvas-basiertes, **MIT-lizenziertes** Open-Source-Grid als Rendering-Schicht, angebunden an unseren Op-/Workbook-State und an HyperFormula. Wir besitzen State und Kollaboration; das Grid ist View + Eingabe.
- **Entkopplung:** schmale interne Schnittstelle `SheetView` (rendere Zellen, melde Edits, zeige Remote-Cursor). Das konkrete Grid ist dahinter austauschbar — kein harter Lock-in, späterer Wechsel ohne Eingriff in den Kollaborations-Kern.

### Modul-Aufteilung

```
sheet/
 ├─ SheetView         (Grid-Rendering + Eingabe, kapselt die Grid-Lib)
 ├─ WorkbookState     (lokales Workbook-Modell: Sheets, Zellen, Style-Pool)
 ├─ FormulaEngine     (HyperFormula-Wrapper: raw rein, value + Abhängigkeiten raus)
 ├─ SheetCollabClient (Ops senden/empfangen, optimistisch anwenden, Server-Ack reconcilen)
 ├─ SheetMessageTypes (Op-Serialisierung, konzeptionell geteilt mit dem Go-Handler)
 └─ Toolbar/Formula-Bar (Eingabe, Zahlenformat, Stil-Buttons)
```

Datenfluss im Client: **Eingabe → WorkbookState (Op) → FormulaEngine (value) → SheetView (render)**, parallel **Op → SheetCollabClient → Server**. Eingehende Remote-Ops laufen denselben Pfad ab „WorkbookState".

### UX-Details v1

- Sichtbare Präsenz: farbige Zell-Cursor/Selektionen anderer Nutzer (Author-Farben aus dem Bestand)
- Toolbar: Zahlenformat, Schrift/Fett/Kursiv/Farbe, Ausrichtung, Rahmen, Zeile/Spalte einfügen/löschen, Sheet-Tabs unten
- Formel-Leiste oben mit Zellreferenz-Anzeige
- Read-only-Modus über die vorhandene Readonly-ID

### Leitgedanke

Das Grid ist eine **austauschbare View hinter einer schmalen Schnittstelle**; State, Formeln und Kollaboration gehören uns und bleiben Grid-unabhängig.

---

## 6. xlsx Import/Export

### Bibliothek: `excelize` (Go)

`github.com/xuri/excelize/v2` (BSD-3, kein Lizenzproblem), serverseitig — kein Browser/JS nötig.

### Import — `POST /s/:pad/import` (multipart-Upload)

```
.xlsx hochladen
  │
  ▼ excelize liest Sheets, Zellen, Stile
1. Map excelize-Modell → internes Workbook-Modell (Abschnitt 3)
     - Zell-raw (Werte & Formeln) übernehmen
     - Stile → Style-Pool deduplizieren → styleId
     - Merged Ranges, Zeilen-/Spaltenmaße, Sheet-Namen
2. Workbook als initialen Zustand persistieren (Head-Rev = 0)
3. Bestehende offene Clients erhalten "Workbook ersetzt"-Event
```

**Grenzen v1:** Werte + Formeln + Basis-Formatierung + Merges + Sheet-Struktur. **Nicht** v1: Charts, Pivot-Tabellen, eingebettete Bilder, Makros, bedingte Formatierung, Data Validation. Unbekannte Inhalte werden **verlustfrei übersprungen mit Warnhinweis**, nicht hart abgelehnt.

### Export — `GET /s/:pad/export.xlsx`

```
internes Workbook → excelize-Builder → .xlsx-Download
  - Zellen: raw (Formeln bleiben Formeln) + value als Cache
  - Style-Pool → excelize-Stile
  - Merges, Maße, Sheet-Reihenfolge
```

- Roundtrip-Ziel: Import→Export eines unterstützten Dokuments bleibt strukturell stabil (getestet).
- Formeln werden **als Formeln** exportiert (nicht als statische Werte) — Excel rechnet sie beim Öffnen neu; der `value`-Cache dient als Fallback-Anzeige.

---

## 7. Test-Strategie (TDD)

- **Go-Unit-Tests:**
  - Op-Anwendung & Reihenfolge: Zell-Ops kommutieren; Struktur-Ops Index-Transformation gegen veraltete `rev` — **heikelste Logik, umfangreichste Abdeckung hier**
  - Style-Pool-Deduplizierung
  - xlsx-Roundtrip: Fixture-.xlsx → Import → internes Modell → Export → erneut lesen, Felder vergleichen
  - Snapshot/Replay: Op-Log → Zustand == direkter Zustand
- **Frontend-Unit-Tests:** WorkbookState-Op-Anwendung, FormulaEngine-Wrapper (raw→value), Op-Serialisierung Round-trip
- **Playwright-E2E** (vorhandenes `/playwright`): zwei Browser-Sessions, gleichzeitige Edits in verschiedenen/gleichen Zellen → Konvergenz; Formel-Neuberechnung über Clients; Reconnect mit Op-Nachlauf; xlsx-Export-Download
- **Konvergenz-Property-Test:** zufällige Op-Sequenzen aus N simulierten Clients → alle landen im selben Zustand (wichtigster Korrektheits-Nachweis der Kollaboration)

---

## 8. Offene Punkte

1. **HyperFormula-Lizenz (GPLv3)** — bewusst akzeptieren oder MIT-Alternative wählen? (Blockierend für die Engine-Wahl.)
2. **Konkrete Canvas-Grid-Library** — Auswahl als erster Implementierungsschritt, hinter der `SheetView`-Schnittstelle (unkritisch, austauschbar).
3. **Serverseitige Formel-Neuberechnung** — out-of-scope v1, als Folge-Teilprojekt notiert.

---

## 9. Leitprinzipien (Zusammenfassung)

- Ein gemeinsames Dokument-Fundament (Identität, Zugriff, Transport, Präsenz), pro Typ ein eigenes Inhalts-/Op-Modell.
- Zellen sind atomare Einheiten mit totaler Server-Ordnung → einfache Kollaboration, maximale Wiederverwendung der vorhandenen Infrastruktur.
- `raw`/Formeln sind die Wahrheit; `value` wird lokal abgeleitet; excelize macht den xlsx-Transport.
- Die kollaborative Op-Logik ist der am gründlichsten getestete Teil.
- Der Text-Pfad bleibt unangetastet und abwärtskompatibel.
