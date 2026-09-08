# FlashGate MCP – Datei- und Produktmetadaten: Entscheidungen

## Zweck

Dieses Dokument enthält die dauerhaften Produkt-, Architektur- und
Implementierungsentscheidungen für die Datei- und Produktmetadaten
von FlashGate MCP unter Windows und Linux.

Es ist vom temporären Feature-Backlog getrennt und bleibt nach
Abschluss des Features als Projektdokumentation erhalten.

<!-- FP-DECISION-PRODUCT-IDENTITY:BEGIN -->
## DEC-FP-001 – Hersteller, Copyright und Anwendungssymbol

**Status:** Accepted
**Datum:** 2026-07-21

### Entscheidung

| Eigenschaft | Verbindlicher Wert |
|---|---|
| Hersteller / `CompanyName` | `Thomas Weidner` |
| Copyright-Inhaber | `Thomas Weidner` |
| Anwendungssymbol | Font Awesome `bolt-lightning` |
| Font-Awesome-Stil | Classic Solid |
| Icon-Einsatz | Windows-Anwendungssymbol und weitere technisch geeignete Release-Darstellungen |

### Copyright-Format

Für Release-Builds wird folgender Grundwert verwendet:

```text
Copyright © <Jahr> Thomas Weidner
```

Das konkrete Jahr oder ein Jahresbereich wird aus der noch
festzulegenden reproduzierbaren Quell- beziehungsweise Releasezeit
abgeleitet. Die lokale Build-Uhrzeit ist dafür nicht maßgeblich.

### Icon-Quelle und Verarbeitung

- Verwendet wird das Font-Awesome-Icon `bolt-lightning` im Stil Classic Solid.
- Die vom Benutzer verwendete Bezeichnung `solid bold-lightning` wird technisch auf diesen Icon-Identifier abgebildet.
- Die konkrete Font-Awesome-Version wird vor der Aufnahme der Quelldatei festgeschrieben.
- Es wird eine fest versionierte Vektorquelle verwendet.
- Es werden keine Fontdateien in das Repository oder in Release-Artefakte aufgenommen.
- Das Vektorbild wird deterministisch in die erforderlichen Windows-ICO-Größen konvertiert.
- Quelle, Version, Lizenz und gegebenenfalls erforderliche Attribution werden dokumentiert.
- Die Icon-Erzeugung darf bei normalen oder reproduzierbaren Builds keine Netzwerkverbindung benötigen.

### Abgrenzung

Diese Entscheidung umfasst keine:

- Authenticode-Signatur,
- Markenregistrierung,
- Änderung des MCP-Implementierungsnamens,
- Änderung des Binary-Namens,
- Änderung der Softwarelizenz.

### Backlog-Zuordnung

- Primär: `BL-246`
<!-- FP-DECISION-PRODUCT-IDENTITY:END -->

<!-- FP-DECISION-IDENTITY-VERSION:BEGIN -->
## DEC-FP-002 – Produktidentität, Versionierung und reproduzierbare Buildzeit

**Status:** Accepted
**Datum:** 2026-07-21

### Produktidentität

| Eigenschaft | Verbindlicher Wert |
|---|---|
| Produktname | `FlashGate MCP` |
| Binary-Name | `flashgate-mcp` |
| Windows-Originaldateiname | `flashgate-mcp.exe` |
| Interner Name | `flashgate-mcp` |
| MCP-Implementierungsname | `flashgate` |
| Dateibeschreibung | `FlashGate MCP Server` |
| Hersteller | `Thomas Weidner` |
| Copyright-Inhaber | `Thomas Weidner` |
| Lizenz | `GNU General Public License v3.0` |
| Projektadresse | `https://github.com/thomasweidner/flashgate-mcp` |
| Kommentar | `Native Model Context Protocol server for controlled local system access.` |
| Windows-Ressourcensprache | Englisch – Vereinigte Staaten (`0x0409`) |
| Windows-Zeichensatz | Unicode (`1200`) |

Der MCP-Implementierungsname `flashgate` und der Binary-Name
`flashgate-mcp` werden durch dieses Feature nicht geändert.

### Kanonische Versionsquelle

- Release-Versionen stammen aus einem Git-Tag.
- Release-Tags verwenden das Format `vMAJOR.MINOR.PATCH`.
- Prerelease-Tags dürfen einen SemVer-Suffix enthalten, zum Beispiel
  `v0.5.0-rc.1`.
- Das führende `v` gehört zum Git-Tag, aber nicht zur eingebetteten
  Produktversion.
- Außerhalb eines Release-Tags wird die Entwicklungsproduktversion
  `0.0.0-dev` verwendet.
- Manuell frei eingegebene Releaseversionswerte sind nicht die
  kanonische Quelle.

### Semantic Versioning

- Produkt- und Releaseversionen folgen Semantic Versioning.
- Die vollständige Produktversion kann beispielsweise
  `0.5.0`, `0.5.0-rc.1` oder `0.5.0+build.1` lauten.
- Ungültige SemVer-Werte müssen vor einem Release-Build
  abgelehnt werden.

### Windows-Dateiversion

- Die numerische Windows-Dateiversion verwendet vier Komponenten:
  `Major.Minor.Patch.0`.
- Beispiel: Produktversion `0.5.0-rc.1` ergibt die numerische
  Dateiversion `0.5.0.0`.
- Prerelease- und Build-Suffixe erscheinen ausschließlich in
  `ProductVersion` und gegebenenfalls den textuellen Versionsfeldern.
- Die vierte numerische Komponente bleibt für dieses Feature `0`.

### Git-Revision und Dirty-Status

- Intern wird die vollständige 40-stellige Git-Commit-SHA verwendet.
- Eine kompakte menschenlesbare Ausgabe darf die ersten zwölf
  hexadezimalen Zeichen anzeigen.
- Der Dirty-Zustand wird als eigenes boolesches Feld `Modified`
  beziehungsweise gleichwertig dargestellt.
- Der Dirty-Zustand ist niemals Bestandteil der numerischen
  Windows-Dateiversion.
- Ein Release-Build mit unerwartetem Dirty-Zustand muss
  fail-closed abgelehnt werden.

### Kanonische Zeitquelle

- Alle eingebetteten Build- und Quellzeitpunkte verwenden UTC.
- Das kanonische Textformat ist RFC 3339 mit dem Suffix `Z`,
  beispielsweise `2026-07-21T16:38:27Z`.
- Falls `SOURCE_DATE_EPOCH` gesetzt ist, ist dieser Wert führend.
- Andernfalls wird die Git-Commit-Zeit des gebauten Commits verwendet.
- Die lokale Systemzeit des Build-Hosts wird nicht als kanonischer
  Binary-Zeitstempel verwendet.
- Eine feste Zeitverschiebung `GMT+1` oder `CET` wird nicht
  eingebettet, weil sie Sommerzeitperioden nicht zuverlässig abbildet.
- Menschliche Berichte dürfen zusätzlich eine lokale Zeit anzeigen,
  sofern Zeitzone und UTC-Offset ausdrücklich angegeben werden.

### Reproduzierbarkeit und Datenschutz

- Gleiche Quell-, Versions- und Zeitinputs müssen identische
  Produktmetadaten erzeugen.
- Hostname, Benutzername, lokaler Arbeitsverzeichnispfad,
  OneDrive-Pfad und andere maschinenspezifische Werte werden nicht
  in Release-Metadaten eingebettet.
- Buildzeitpunkte dürfen nicht bei jedem identischen Rebuild neu
  aus der lokalen Uhr erzeugt werden.
- Windows-Ressource, CLI, Go-Buildinformationen, Release-Artefaktname
  und spätere Paketmetadaten müssen dieselbe Version verwenden.

### Zum Zeitpunkt von DEC-FP-002 noch offene technische Umsetzung

Die folgenden Punkte waren bei Annahme von DEC-FP-002 noch offen und wurden anschließend verbindlich in `DEC-FP-003` entschieden:

DEC-FP-002 legte noch nicht fest:

- den konkreten Windows-Ressourcengenerator,
- ob eine `.syso`-Datei committet oder ausschließlich erzeugt wird,
- das exakte Verhältnis zwischen Linkerwerten und Go-`-buildvcs`,
- die Go-/ELF-Build-ID-Strategie,
- den Umfang von `.deb`, `.rpm` und systemd-Artefakten,
- das endgültige CLI-Format für ausführliche oder JSON-Ausgaben.

### Backlog-Zuordnung

- Primär: `BL-246`
- Gemeinsamer Begleitumfang: `BL-247`
<!-- FP-DECISION-IDENTITY-VERSION:END -->

<!-- FP-DECISION-TECHNICAL-IMPLEMENTATION:BEGIN -->
## DEC-FP-003 – Technische Metadaten-, Build- und Architekturstrategie

**Status:** Accepted
**Datum:** 2026-07-21

### Single Source of Truth

- Alle Buildparameter werden einmal zentral ermittelt.
- Die kanonischen Werte umfassen Produktversion, Windows-Dateiversion,
  vollständige Commit-SHA, Quellzeit in UTC, Dirty-Status, GOOS und GOARCH.
- Diese Werte versorgen Go-Linkerparameter, CLI, Windows-Ressource,
  Release-Artefaktnamen und spätere Paketmetadaten.
- Go-VCS-Daten dienen als unabhängige Provenienz- und Konsistenzquelle.
- Widersprüche zwischen expliziten Buildwerten und Go-VCS-Daten führen
  bei kontrollierten Release-Builds zum Abbruch.
- Ein direkt im Binary eingebettetes, maschinenlesbares Buildmanifest
  bindet dieselben Werte statisch. Es ist insbesondere die
  architekturunabhängige Provenienzquelle für Windows ARM64, wenn das
  Binary auf dem prüfenden x64-Host nicht ausgeführt werden kann.

### Build- und Laufzeitinformationen

- Produktidentität, Produktversion, Commit und Quellzeit sind Builddaten.
- Go-Version, GOOS und GOARCH werden aus der Go-Laufzeit oder
  `debug.ReadBuildInfo` gelesen.
- Hostname, Benutzername, lokale Pfade und andere Hostinformationen
  werden nicht eingebettet.

### Gemeinsames Build-Info-Schema

| Feld | Bedeutung |
|---|---|
| `ProductName` | `FlashGate MCP` |
| `BinaryName` | `flashgate-mcp` |
| `Version` | vollständige SemVer-Produktversion |
| `FileVersion` | vierteilige numerische Windows-Version |
| `Commit` | vollständige Git-SHA |
| `SourceTime` | RFC-3339-Zeitpunkt in UTC |
| `Modified` | Dirty-Status |
| `GoVersion` | verwendete Go-Toolchain |
| `GOOS` | technisches Go-Zielbetriebssystem |
| `GOARCH` | technische Go-Zielarchitektur |
| `PublicArch` | anwenderorientierte Architekturkennung |
| `BuildManifest` | statische Bindung der kanonischen Buildwerte im Binary |

### Entwicklungsdefaults

- Version: `0.0.0-dev`
- Windows-Dateiversion: `0.0.0.0`
- Commit: `unknown`, falls keine Linker- oder Go-VCS-Daten verfügbar sind
- Quellzeit: `unknown`, falls keine Linker- oder Go-VCS-Daten verfügbar sind
- Plattform und Go-Version werden zur Laufzeit ermittelt.

### Windows-Ressourcengenerator

- Verwendet wird `github.com/josephspurrier/goversioninfo` in der
  fest gepinnten Version `v1.7.0`.
- Es wird kein `@latest` in Build- oder Releaseabläufen verwendet.
- Ein repositoryeigener Generator-Wrapper erzeugt die Ressource aus
  den zentral ermittelten Buildparametern.
- `rc.exe` und GNU `windres` sind keine Pflichtabhängigkeiten.
- Der normale Build benötigt keinen Netzwerkzugriff.

### Windows-Ressourcenfelder

- `FileDescription`: `FlashGate MCP Server`
- `FileVersion`: numerische vierteilige Version
- `ProductName`: `FlashGate MCP`
- `ProductVersion`: vollständige SemVer-Version
- `CompanyName`: `Thomas Weidner`
- `LegalCopyright`: reproduzierbarer Copyright-Text
- `OriginalFilename`: `flashgate-mcp.exe`
- `InternalName`: `flashgate-mcp`
- `Comments`: `Native Model Context Protocol server for controlled local system access.`

`PrivateBuild`, `SpecialBuild`, `LegalTrademarks` und ähnliche
Zusatzfelder werden ohne konkreten Bedarf nicht verwendet.

### Icon- und Branding-Artefakte

- Das fest versionierte Font-Awesome-SVG und die daraus erzeugte
  ICO-Datei werden als überprüfte Branding-Artefakte committet.
- Fontdateien werden nicht übernommen.
- Quelle, Version, Lizenz, Attribution und Hashes werden dokumentiert.
- Die ICO-Datei ist nicht versionsabhängig und wird nicht bei jedem
  normalen Build neu erzeugt.
- Die Prüfung normalisiert alle eingebetteten Iconframes und vergleicht
  deren Deskriptoren und SHA-256-Werte mit der committeten ICO-Datei.

### `.syso`-Lebenszyklus

- `.syso`-Dateien werden nicht committet.
- Sie werden unmittelbar vor dem Windows-Build erzeugt.
- Die Dateinamen enthalten Zielbetriebssystem und technische
  Go-Architektur, beispielsweise `resource_windows_amd64.syso`
  oder `resource_windows_arm64.syso`.
- Der Build lehnt unerwartete oder veraltete `.syso`-Dateien ab.
- Generierte Dateien werden auch bei Fehlern entfernt.
- Generierte `.syso`-Dateien werden über `.gitignore` ausgeschlossen.

### Go-VCS-Strategie

- Direkte lokale Entwicklerbuilds verwenden `-buildvcs=auto`.
- Kontrollierte Skript-, CI- und Release-Builds verwenden
  `-buildvcs=true`.
- Release-Builds mit Dirty-Status werden abgelehnt.
- Explizite Buildwerte sind die anwenderorientierte Quelle.
- Go-VCS-Daten werden für Fallback, Provenienz und
  Konsistenzprüfung verwendet.
- Das statische Buildmanifest wird aus denselben bereits validierten
  Werten wie CLI und Linkerfelder erzeugt und direkt aus Windows- und
  Linux-Binaries geprüft; eine ungeprüfte Text-Sidecar-Datei ist keine
  Provenienzquelle.

### Go- und ELF-Build-ID

- Die Go-Standard-Build-ID wird beibehalten.
- Es wird keine eigene `-buildid`-Zeichenfolge vorgegeben.
- `go tool buildid -w` wird nicht im Releaseprozess verwendet.
- Windows- und Linux-Artefakte werden mit `go tool buildid` geprüft.
- Linux wird zusätzlich nativ mit `readelf -n` geprüft.
- Es wird keine zusätzliche GNU-Build-ID erzwungen.

### CLI

- `flashgate-mcp --version` liefert `flashgate-mcp <version>`.
- `flashgate-mcp --version --verbose` liefert Produkt, Version,
  Dateiversion, Commit, Quellzeit, Dirty-Status, Go-Version,
  öffentliche Plattform und technisches Go-Ziel.
- Eine JSON-Versionsausgabe wird in diesem Feature nicht implementiert.

Beispiel für x64:

```text
Platform:  windows/x64
Go target: windows/amd64
```

Beispiel für ARM64:

```text
Platform:  windows/arm64
Go target: windows/arm64
```

### Interne und öffentliche Architekturbezeichnungen

| GOARCH | Öffentliche Kennung | Benutzerbeschreibung |
|---|---|---|
| `amd64` | `x64` | 64 Bit für Intel- oder AMD-Prozessoren |
| `arm64` | `arm64` | ARM 64 Bit |

- Interner Go-Code, Buildskripte und `.syso`-Dateinamen verwenden
  weiterhin die vorgegebenen Werte `amd64` und `arm64`.
- Öffentliche Dateinamen, Downloadbeschreibungen und anwenderorientierte
  Ausgaben verwenden `x64` beziehungsweise `arm64`.
- `x86_64` und `aarch64` werden nicht als öffentliche
  FlashGate-Kennungen verwendet.

### Release-Artefakte

- Windows x64:
  `flashgate-mcp_<version>_windows_x64.zip`
- Linux x64:
  `flashgate-mcp_<version>_linux_x64.tar.gz`
- Windows ARM64:
  `flashgate-mcp_<version>_windows_arm64.zip`
- Linux ARM64:
  `flashgate-mcp_<version>_linux_arm64.tar.gz`
- Das führende `v` des Git-Tags wird nicht in Produktversion oder
  Archivnamen übernommen.
- Archive enthalten Binary, `LICENSE`, `README.md` und
  `THIRD-PARTY-NOTICES.md`.
- SHA-256-Prüfsummen werden neben den Archiven veröffentlicht.
- Vor jedem Upload werden zwei unabhängige Builds einschließlich Binary,
  Archiv, Prüfsummendatei und exaktem Inventar verglichen.
- Ein maschinenlesbarer Leak-Scan aller Binär- und Archivinhalte muss
  vor dem Upload bestehen.

### Linux-Paketumfang

- Das native ELF-Binary und ein versionierter Tarball werden umgesetzt.
- `.deb` und `.rpm` werden bis zu einer realen Distributionsanforderung
  zurückgestellt.
- systemd-Metadaten werden zusammen mit der späteren
  systemd-Serviceimplementierung umgesetzt.
- Extended Attributes bleiben ausgeschlossen.

### Zielarchitekturen

| Go-Ziel | Öffentliches Artefakt | Native Prüfung | Anfangsstatus |
|---|---|---|---|
| `windows/amd64` | `windows_x64` | Windows-x64-Runner | Stable |
| `linux/amd64` | `linux_x64` | Ubuntu-x64-Runner | Stable |
| `windows/arm64` | `windows_arm64` | x64-Runner: Cross-Build und statische Prüfung; nativer ARM64-Runner: Zielzustand | Preview |
| `linux/arm64` | `linux_arm64` | x64-Runner: Cross-Build und statische Prüfung; nativer ARM64-Runner: Zielzustand | Preview |

- ARM64-Artefakte können ohne lokale ARM-Hardware cross-kompiliert werden.
- Ein erfolgreicher Cross-Build allein gilt nicht als native Validierung.
- Der aktuell implementierte Runner-Modus baut ARM64 auf x64-Hosts und prüft
  die Binärdateien statisch. Er führt sie nicht nativ aus und liefert keinen
  Nachweis für native Windows- oder Ubuntu-ARM64-Ausführung.
- Native ARM64-Tests über geeignete Windows- und Ubuntu-ARM64-Runner sind ein
  zukünftiger, von der Verfügbarkeit solcher Runner abhängiger Zielzustand.
- ARM64 wird erst nach wiederholten erfolgreichen Releasezyklen und
  stabiler Runnerverfügbarkeit auf Stable hochgestuft.
- Weitere Architekturen werden nur bei konkretem Bedarf und mit
  eigener nativer Validierungsstrategie aufgenommen.
- Bei einer späteren Einführung von cgo oder nativen Bibliotheken
  wird die Cross-Compilation neu bewertet.

### Testmatrix

- Windows x64 Stable
- Linux x64 Stable
- Windows ARM64 Preview
- Linux ARM64 Preview
- Entwicklungsbuild ohne Release-Tag
- Stable- und Prerelease-SemVer
- Clean und Dirty Working Tree
- Windows-Dateiversionsabbildung
- CLI-Kurz- und Verbose-Ausgabe
- Windows-`VERSIONINFO`
- Go-VCS- und Build-ID-Konsistenz
- natives `readelf -n` unter Linux
- gemeinsame SemVer-/`SOURCE_DATE_EPOCH`-Fixturematrix
- manipulierte TAR-Typ-, Traversal- und Inventarfixtures
- manipulierte Icon-Identitätsfixture
- reguläres Repository und Linked Worktree

### Backlog-Zuordnung

- Primär: `BL-246`
- Gemeinsamer Begleitumfang: `BL-247`
<!-- FP-DECISION-TECHNICAL-IMPLEMENTATION:END -->
