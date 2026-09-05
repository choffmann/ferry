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

## Referenzlösungen

Zu jedem Block gehört ein Branch `solutions/block-NN`, der nach dem Termin erscheint.
