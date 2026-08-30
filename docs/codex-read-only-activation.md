# Codex read-only activation preparation

Diese Anleitung bereitet eine spätere Aktivierung vor. `SPR-044` aktiviert FlashGate nicht automatisch in Codex.

Die reale Aktivierung darf erst nach Review, Commit, Pull Request, Merge und Post-Merge-Prüfung erfolgen und benötigt eine separate Bestätigung. Die Beispiele auf dieser Seite wurden nicht auf eine reale Codex-, Claude-Desktop- oder andere Clientkonfiguration angewendet.

## 1. Voraussetzungen

- ein gemergter und geprüfter FlashGate-Commit;
- ein reproduzierbar gebautes Windows- oder Linux-Binary;
- ein kleiner, ausdrücklich freigegebener absoluter Root;
- erfolgreiche Default-, Read-only-, Negative- und Startup-Negativ-Smokes, deren positive Antworten `CallToolResult.content[]` und `structuredContent` validieren;
- Backup der bestehenden Clientkonfiguration;
- dokumentierter Rollback.

Die erste Aktivierung verwendet kein `go run` und keinen automatischen Build beim MCP-Start.

## 2. Binary bauen und prüfen

Das erste Aktivierungsbinary wird vom exakten gemergten `main`-Commit mit `-trimpath` gebaut. Vor einer späteren Konfigurationsänderung sind mindestens zu prüfen:

```powershell
git rev-parse HEAD
go build -trimpath -o build\flashgate-mcp.exe ./cmd/server
.\build\flashgate-mcp.exe --help
.\build\flashgate-mcp.exe --version
Get-FileHash -Algorithm SHA256 .\build\flashgate-mcp.exe
```

Das geprüfte Binary wird danach unter separater Freigabe in einen versionierten Pfad außerhalb des Repositories kopiert, zum Beispiel:

```text
C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Bin\FlashGate\spr-44\flashgate-mcp.exe
```

Der Binary-Pfad darf nicht innerhalb des freigegebenen Datenroots liegen. Temporäre Builddateien sind nach der Abnahme zu entfernen. Ein geprüftes Release-Binary bleibt die spätere bevorzugte Produktionsform.

## 3. Sicheren Root wählen

`MCP_ROOT` ist verpflichtend. Der produktive Root muss absolut, vorhanden, zugreifbar und ein Verzeichnis sein.

Für die erste Abnahme wird ein kleiner dedizierter Root empfohlen, beispielsweise:

```text
C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\FlashGate-ReadOnly-TestRoot
```

Nicht verwenden:

- ein gesamtes Laufwerk;
- das Home-Verzeichnis;
- den gesamten Voxtronic- oder OneDrive-Baum;
- einen relativen Root;
- `.` ohne ausdrücklichen Development-Opt-in;
- einen Binary- oder Buildordner als Datenroot.

Ein OneDrive-Root muss lokal verfügbar sein und die Hidden-, UNC-, Symlink-, Junction- und Reparse-Policy bestehen.

## 4. Verbindliches Read-only-Profil

Für Codex muss ausdrücklich gesetzt werden:

```text
MCP_READ_ONLY=true
```

Fehlt die Variable, bleibt das normale Default-Profil mit acht Tools aktiv. Das ist beabsichtigt, aber für die erste Codex-Aktivierung nicht zulässig.

Zusätzliche sichere Werte:

```text
MCP_ALLOW_CWD_ROOT=false
MCP_ALLOW_HIDDEN_FILES=false
MCP_ALLOW_UNC_PATHS=false
MCP_FOLLOW_SYMLINKS=false
MCP_DEBUG=false
```

Das Read-only-Profil exponiert exakt in dieser Reihenfolge:

1. `list_directory`
2. `read_file`
3. `get_path_info`

## 5. Windows-Codex-Beispiel – nicht automatisch anwenden

Lokal bestätigt sind `command`, `args`, Environment, `startup_timeout_sec` und `codex mcp add`. Die manuelle `cwd`-Syntax und `tool_timeout_sec` waren während `SPR-044` nicht abschließend lokal bestätigt und müssen unmittelbar vor einer realen Aktivierung gegen die dann installierte Codex-Version geprüft werden.

```toml
# EXAMPLE ONLY — NOT APPLIED BY SPR-044
[mcp_servers.flashgate_readonly]
command = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Bin\FlashGate\spr-44\flashgate-mcp.exe'
args = []
startup_timeout_sec = 10

# Verify before use with the installed Codex version:
# cwd = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\Bin\FlashGate\spr-44'
# tool_timeout_sec = 30

[mcp_servers.flashgate_readonly.env]
MCP_ROOT = 'C:\Users\ThomasW\OneDrive - VOXTRONIC\Desktop\Voxtronic\Codex-Work\FlashGate-ReadOnly-TestRoot'
MCP_READ_ONLY = 'true'
MCP_ALLOW_CWD_ROOT = 'false'
MCP_ALLOW_HIDDEN_FILES = 'false'
MCP_ALLOW_UNC_PATHS = 'false'
MCP_FOLLOW_SYMLINKS = 'false'
MCP_DEBUG = 'false'
```

`codex mcp add` ist lokal als Mechanismus bestätigt. Es wird in `SPR-044` ausdrücklich nicht ausgeführt. Vor einer späteren Nutzung sind Backup, exakter Name, Umgebungswerte, Timeouts und resultierender Konfigurationsdiff zu prüfen.

## 6. Linux- und spätere WSL-Perspektive

Ein Linux-Binary benötigt Execute-Berechtigung und einen absoluten Linux-Root:

```text
/opt/flashgate/spr-44/flashgate-mcp
/home/example/flashgate-readonly-root
```

Pfade mit Leerzeichen müssen als einzelne Konfigurationswerte übergeben werden. Symlinks bleiben standardmäßig deaktiviert. WSL2 wird in `SPR-044` nicht installiert und ist keine Voraussetzung. Eine spätere WSL-Aktivierung benötigt eigene Pfad-, Berechtigungs-, Binary- und Rollbackprüfung; Windows- und WSL-Pfade dürfen nicht stillschweigend gemischt werden.

## 7. Claude Desktop und allgemeines STDIO-Beispiel

Diese Beispiele sind ebenfalls nur Vorbereitung. Die aktuelle Syntax des jeweiligen Clients muss vor Anwendung geprüft werden.

Claude-Desktop-orientiertes Windows-Beispiel:

```json
{
  "mcpServers": {
    "flashgate_readonly": {
      "command": "C:\\Path\\To\\Verified\\flashgate-mcp.exe",
      "args": [],
      "env": {
        "MCP_ROOT": "C:\\Path\\To\\Small\\Approved\\Root",
        "MCP_READ_ONLY": "true",
        "MCP_ALLOW_CWD_ROOT": "false",
        "MCP_ALLOW_HIDDEN_FILES": "false",
        "MCP_ALLOW_UNC_PATHS": "false",
        "MCP_FOLLOW_SYMLINKS": "false"
      }
    }
  }
}
```

Claude-Desktop-orientiertes Linux-Beispiel:

```json
{
  "mcpServers": {
    "flashgate_readonly": {
      "command": "/opt/flashgate/spr-44/flashgate-mcp",
      "args": [],
      "env": {
        "MCP_ROOT": "/home/example/flashgate-readonly-root",
        "MCP_READ_ONLY": "true",
        "MCP_ALLOW_CWD_ROOT": "false",
        "MCP_ALLOW_HIDDEN_FILES": "false",
        "MCP_ALLOW_UNC_PATHS": "false",
        "MCP_FOLLOW_SYMLINKS": "false"
      }
    }
  }
}
```

Allgemeiner lokaler STDIO-Vertrag:

```text
transport = stdio
command = absolute path to verified flashgate-mcp binary
args = empty
environment.MCP_ROOT = absolute approved directory
environment.MCP_READ_ONLY = true
environment.MCP_ALLOW_CWD_ROOT = false
```

Kein Beispiel enthält Tokens, Passwörter oder Authentifizierungswerte.

## 8. Aktivierungs- und Abnahmetests

Nach einer später separat bestätigten Aktivierung:

1. Vor jeder Konfigurationsänderung den direkten STDIO-Preflight mit einem strikten `CallToolResult`-Decoder ausführen; gültiges JSON und Fachfelder allein reichen nicht.
2. MCP-Erkennung und `serverInfo.name=flashgate` prüfen.
3. Exakt drei Tools in der dokumentierten Reihenfolge prüfen.
4. Root und Unterverzeichnis listen.
5. normale, Leerzeichen- und Unicode-Dateien lesen.
6. Datei-, Verzeichnis- und Missing-Metadaten prüfen.
7. Traversal und absolute Outside-Root-Pfade ablehnen.
8. alle Write- und Legacy-Namen negativ prüfen.
9. stdout auf ausschließlich JSON-RPC prüfen.
10. stderr auf sichere Kategorien und fehlende Hostpfade prüfen.
11. Root und Repository auf neue Dateien prüfen.
Write-Namen:

```text
write_file
create_directory
delete_path
copy_path
move_path
```

Legacy-Namen:

```text
list_files
stat_path
exists_path
mkdir
rename_path
```

Alle zehn Namen müssen denselben generischen Invalid-Params-Vertrag liefern.

## 9. Nicht-technische Read-only-Validierung

Für Prüfer ohne Go-Workflow:

1. Binary-Herkunft, Pfad, `--version` und SHA-256 mit dem Freigabeprotokoll vergleichen.
2. Rootpfad auf den kleinen genehmigten Ordner begrenzen.
3. `MCP_READ_ONLY=true` und `MCP_ALLOW_CWD_ROOT=false` sichtbar bestätigen.
4. bereitgestellte PowerShell- oder Bash-Smokes ausführen.
5. Ergebnislisten auf exakt drei Tools prüfen.
6. einen Write-Versuch nur über die Negativtests ausführen und generischen Fehler erwarten.
7. Root vor/nach dem Test vergleichen.
8. Rollbackfähigkeit bestätigen.

Ein Prüfer muss weder `go run` noch einen Build bei jedem Clientstart ausführen.

## 10. Sichere Startup-Kategorien und Troubleshooting

| Kategorie | Bedeutung | Sichere Reaktion |
| --- | --- | --- |
| `missing_root` | `MCP_ROOT` fehlt | expliziten absoluten Root konfigurieren |
| `invalid_root` | leer, Whitespace, relativ oder nicht erlaubtes `.` | Rootwert korrigieren; kein implizites CWD verwenden |
| `root_not_found` | Root existiert nicht | Pfad und lokale Verfügbarkeit prüfen |
| `root_not_directory` | Root ist eine Datei | vorhandenes Verzeichnis wählen |
| `root_not_allowed` | Permission oder bestehende Rootpolicy lehnt ab | Policy und Rootwahl prüfen, keine Policy umgehen |
| `invalid_profile` | ungültiges `MCP_READ_ONLY` | gültigen booleschen Wert setzen; für Codex `true` |
| `invalid_development_option` | ungültiges `MCP_ALLOW_CWD_ROOT` | exakt `true` oder `false` kleingeschrieben verwenden |
| `startup_failed` | unerwarteter Bootstrapfehler | sichere Logs/Umgebung prüfen und Aktivierung zurückrollen |

Startupfehler verwenden Exitcode 3 für erwartbare Konfigurations-/Rootfehler und Exitcode 1 für unerwartete Fehler. stdout bleibt leer; stderr enthält keine rohen OS-Fehler oder absoluten Rootpfade.

## 11. Backup und Rollback

Vor einer späteren Änderung:

1. Codex-/Clientkonfiguration timestamped sichern.
2. vorhandene MCP-Einträge und Version dokumentieren.
3. sicherstellen, dass das Backup keine ungeschützten Secrets in Reports kopiert.

Rollback:

1. FlashGate-Eintrag mit der vom Client unterstützten Methode deaktivieren oder entfernen beziehungsweise das geprüfte Backup wiederherstellen.
2. Keine anderen MCP-Blöcke verändern.
3. Client vollständig neu starten.
4. MCP-Liste prüfen; FlashGate darf nicht aktiv sein.
5. prüfen, dass kein `flashgate-mcp`-Prozess verbleibt.
6. Root und Repository auf Artefakte prüfen.
7. Binary nur nach separater Freigabe entfernen.

`SPR-044` führt weder `codex mcp add` noch `codex mcp remove` aus und verändert keine reale `config.toml` oder Auth-Datei.

## 12. CallToolResult- und Canary-Gate

Der erste Codex-Canary wurde nach gültigem JSON-RPC-Preflight aktiviert, scheiterte aber im echten Client mit `Unexpected response type`, weil erfolgreiche Fachobjekte ungewrappt waren. `SPR-045` korrigiert den Serververtrag, reaktiviert den Canary jedoch nicht.

Eine Reaktivierung ist erst nach Review, Commit, PR, vollständig grüner Windows-/Ubuntu-CI, Merge, Post-Merge-Gates, neuem versioniertem Binary, strengem direkten Preflight, separater Benutzerfreigabe, vollständigem Codex-Neustart und erfolgreicher Wiederholung des modellgestützten End-to-End-Tests zulässig. Der bestehende Canary bleibt bis dahin deaktiviert.
