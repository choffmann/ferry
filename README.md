# ferry

Demo-Anwendung für das Modul DevOps an der Hochschule Flensburg. Eine Fahrbuchung:
Verbindungen abfragen, Abfahrten mit freien Plätzen finden, buchen, Status abfragen.

Dieses Repo ist der Upstream des Kurses. Es enthält Quellcode, Migrationen, Tests und
Dokumentation. Es enthält bewusst kein Dockerfile, keine CI-Konfiguration, kein Compose
und keine Kubernetes-Manifeste. Das sind die Abgaben der Teams.

## Nutzung im Kurs

Jedes Team arbeitet in einem eigenen Repo und trägt dieses hier als zweite Remote ein:

```sh
git remote add upstream https://github.com/choffmann/ferry.git
git fetch upstream --tags
git merge v0.1.0
```

Neue Releases erscheinen freitags vor dem jeweiligen Termin. Die Release Notes stehen
im GitHub-Release und sind Pflichtlektüre.

## Umgebung prüfen

```sh
curl -fsSL https://raw.githubusercontent.com/choffmann/ferry/main/check-env.sh | sh
```

Wer Nix benutzt, findet im Repo eine `flake.nix` mit einer Entwicklungsumgebung, die
alle nötigen Werkzeuge mitbringt: `nix develop`, oder automatisch per `direnv` über die
mitgelieferte `.envrc`. Sie wächst mit den Releases mit. Nötig ist sie nicht,
`check-env.sh` bleibt der Maßstab.

## Bauen und starten

Die Anwendung braucht eine Postgres-Datenbank. Getestet wird gegen Postgres 17.

```sh
go build -o ferry ./cmd/ferry

export DATABASE_URL='postgres://ferry:ferry@localhost:5432/ferry?sslmode=disable'
export ADMIN_TOKEN='local-dev'

./ferry migrate   # spielt das Schema ein
./ferry seed      # schreibt Verbindungen und Fahrplan
./ferry serve
```

Die drei Schritte gehören in diese Reihenfolge. `migrate` und `seed` sind beide
wiederholbar und beenden sich mit Status 0, auch wenn es nichts zu tun gab.

Ohne erreichbare Datenbank startet `ferry serve` nicht und wartet auch nicht. Wann
die Datenbank bereitsteht, gehört in die Beschreibung der Umgebung.

| Variable | Vorgabe | Bedeutung |
|---|---|---|
| `DATABASE_URL` | keine, Pflicht | Verbindungszeichenkette zur Datenbank |
| `ADMIN_TOKEN` | keine, Pflicht für `serve` | Bearer-Token der Chaos-Schnittstelle |
| `PORT` | `8080` | Port, auf dem der HTTP-Server lauscht |
| `ASSETS_DIR` | `./assets` | Verzeichnis mit den Vorlagen |

Ein so gebautes Binary trägt keine Build-Informationen. `/version` antwortet dann mit
leeren Feldern und beim Start steht eine Warnung im Log. Die drei Werte kommen über
`-ldflags` hinein:

```sh
go build -ldflags "\
  -X github.com/choffmann/ferry/internal/obs.version=$(git describe --tags --always) \
  -X github.com/choffmann/ferry/internal/obs.commit=$(git rev-parse --short HEAD) \
  -X github.com/choffmann/ferry/internal/obs.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o ferry ./cmd/ferry
```

Beim Start löst `ferry serve` die Zeitzone `Europe/Berlin` auf, in der der Fahrplan
steht. Fehlt der Laufzeitumgebung die Zeitzonendatenbank, bricht der Start mit einer
Meldung ab.

Ebenfalls beim Start liest die Anwendung die Vorlagen aus `assets/`. Das Verzeichnis
gehört damit zum Lieferumfang: wo nur das Binary liegt, startet der Prozess nicht.
`ASSETS_DIR` zeigt auf einen anderen Pfad.

Die API ist in `docs/api.md` beschrieben, die Chaos-Schnittstelle in
`docs/chaos.md`, Schema und Migrationen in `docs/database.md`.

## Referenzlösungen

Zu jedem Block gehört ein Branch `solutions/block-NN`, der nach dem Termin erscheint.
