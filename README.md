# ferry

Demo-Anwendung für das Modul DevOps an der Hochschule Flensburg. Eine Fahrbuchung:
Verbindungen abfragen, Abfahrten mit freien Plätzen finden, buchen, Status abfragen.

Seit v0.4.0 wird eine Buchung nicht mehr im Zuge der Anfrage bestätigt. Sie entsteht
als `pending`, ein Worker rechnet sie gegen einen zweiten Dienst ab und setzt sie auf
`confirmed` oder `failed`. Zu betreiben sind damit drei Prozesse statt einem.

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

Neue Releases erscheinen vor dem jeweiligen Termin. Die Release Notes stehen
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

Aus dem Repo entstehen zwei Binaries. `ferry` trägt die API, den Worker und die
Wartungsbefehle, `payments` ist der Zahlungsdienst.

```sh
go build -o ferry ./cmd/ferry
go build -o payments ./cmd/payments

export DATABASE_URL='postgres://ferry:ferry@localhost:5432/ferry?sslmode=disable'
export ADMIN_TOKEN='local-dev'

./ferry migrate   # spielt das Schema ein
./ferry seed      # schreibt Verbindungen und Fahrplan
```

Die beiden Schritte gehören in diese Reihenfolge. `migrate` und `seed` sind beide
wiederholbar und beenden sich mit Status 0, auch wenn es nichts zu tun gab.

Danach laufen drei Prozesse nebeneinander, jeder in einem eigenen Terminal mit
denselben Variablen wie oben:

```sh
PORT=8081 ./payments                                   # der Zahlungsdienst
./ferry serve                                          # die API auf 8080
PAYMENTS_URL='http://localhost:8081' ./ferry worker    # rechnet die Buchungen ab
```

`ferry serve` und `payments` lauschen beide auf dem Port aus `PORT`, im Beispiel
weicht der Zahlungsdienst deshalb auf 8081 aus. Der Worker lauscht auf keinem Port.
Er hat keinen HTTP-Server, keine Sonden und keine Chaos-Schnittstelle. Er braucht die
Datenbank und die Adresse des Zahlungsdienstes.

Die drei Prozesse sind einzeln lauffähig. Läuft kein Worker, bleiben die Buchungen
`pending`. Antwortet der Zahlungsdienst nicht, ebenso.

Ohne erreichbare Datenbank starten weder `ferry serve` noch `ferry worker`, beide
warten auch nicht. Wann die Datenbank bereitsteht, gehört in die Beschreibung der
Umgebung.

| Variable | Vorgabe | Bedeutung |
|---|---|---|
| `DATABASE_URL` | keine, Pflicht für `serve`, `worker`, `migrate` und `seed` | Verbindungszeichenkette zur Datenbank |
| `ADMIN_TOKEN` | keine, Pflicht für `serve` | Bearer-Token der Chaos-Schnittstelle |
| `PAYMENTS_URL` | keine, Pflicht für `worker` | Adresse des Zahlungsdienstes |
| `PORT` | `8080` | Port, auf dem `ferry serve` oder `payments` lauscht |
| `ASSETS_DIR` | `./assets` | Verzeichnis mit den Vorlagen |

`payments` liest von diesen Variablen nur `PORT`.

Ein so gebautes Binary trägt keine Build-Informationen. `/version` antwortet dann mit
leeren Feldern und beim Start steht eine Warnung im Log. Das gilt für beide Binaries,
sie lesen dieselben drei Variablen aus `internal/obs`. Die Werte kommen über
`-ldflags` hinein:

```sh
LDFLAGS="\
  -X github.com/choffmann/ferry/internal/obs.version=$(git describe --tags --always) \
  -X github.com/choffmann/ferry/internal/obs.commit=$(git rev-parse --short HEAD) \
  -X github.com/choffmann/ferry/internal/obs.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"

go build -ldflags "$LDFLAGS" -o ferry    ./cmd/ferry
go build -ldflags "$LDFLAGS" -o payments ./cmd/payments
```

Beide Ziele bekommen dieselben Werte, damit `/version` bei API und Zahlungsdienst
dasselbe Release nennt.

Beim Start löst `ferry serve` die Zeitzone `Europe/Berlin` auf, in der der Fahrplan
steht. Fehlt der Laufzeitumgebung die Zeitzonendatenbank, bricht der Start mit einer
Meldung ab.

Ebenfalls beim Start liest die Anwendung die Vorlagen aus `assets/`. Das Verzeichnis
gehört damit zum Lieferumfang: wo nur das Binary liegt, startet der Prozess nicht.
`ASSETS_DIR` zeigt auf einen anderen Pfad.

Die API ist in `docs/api.md` beschrieben, die Chaos-Schnittstelle in
`docs/chaos.md`, Schema und Migrationen in `docs/database.md`, der Zahlungsdienst
in `docs/payments.md`.

## Referenzlösungen

Zu jedem Block gehört ein Branch `solutions/block-NN`, der nach dem Termin erscheint.
