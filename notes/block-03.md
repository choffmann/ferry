# Block 3: Git und Merge-Konflikte

## Konflikt mit v0.1.1

Der Konflikt entsteht im Konstantenblock von `internal/domain/seed.go`. In Block 2
wurde `SeatsPerDeparture` auf 4 gesetzt, v0.1.1 richtet dieselbe Zeile neu aus und
fügt `BoardingClosesBefore` hinzu:

```
<<<<<<< HEAD
 SeatsPerDeparture = 4
=======
 SeatsPerDeparture    = 40
 BoardingClosesBefore = 15 * time.Minute
>>>>>>> v0.1.1
```

Die Auflösung behält den eigenen Wert und die neue Konstante:

```go
 SeatsPerDeparture    = 4
 BoardingClosesBefore = 15 * time.Minute
```

Mit `--ours` fehlt `BoardingClosesBefore`, der Build bricht in `internal/domain/domain.go`
ab. Mit `--theirs` baut alles, die Kapazität steht aber wieder auf 40.

Vor dem Merge-Commit prüfen:

```bash
go build ./... && go test ./...
git add internal/domain/seed.go
git commit
```

Dieser Lösungsbranch besteht aus einem einzigen Commit auf v0.1.1 und enthält
keinen Merge-Commit. Für die Aufgabe zum Objektmodell braucht ihr euren eigenen
Merge.

## Objektmodell

```bash
git cat-file -p HEAD               # Commit: tree, zwei parent-Zeilen, author, committer
git cat-file -p HEAD^{tree}        # Tree der Wurzel
git cat-file -p HEAD:internal      # Tree eines Unterverzeichnisses
git cat-file -p HEAD:internal/domain/seed.go   # Blob
git cat-file -t <hash>             # Typ eines Objekts
```

Ein Merge-Commit hat zwei Eltern, einen je zusammengeführter Historie. `HEAD^1` ist
der Stand, auf dem gemergt wurde, `HEAD^2` das gemergte Release.

Ändert sich eine Datei, ändert sich der Hash ihres Blobs. Der Tree, der den Blob
referenziert, bekommt dadurch einen neuen Hash, ebenso jeder Tree darüber bis zur
Wurzel. Der Commit referenziert den Wurzel-Tree und erhält damit ebenfalls einen
neuen Hash. Da jeder spätere Commit den Hash seines Vorgängers enthält, ändern sich
auch alle Nachfolger. Deshalb schreibt ein Rebase die Historie neu.

## Branch-Protection

Die Regeln liegen in den Einstellungen des Repositories, nicht im Repository selbst.
Auf GitHub unter Settings, Branches beziehungsweise Rules für `main`:

- Require a pull request before merging
- Required approvals: 1
- Require approval of the most recent reviewable push
- Require review from Code Owners
- Do not allow bypassing the above settings
- Force-Pushes und Löschen des Branches nicht erlauben

Auf GitLab entspricht das Protected Branches (Allowed to push: No one) zusammen
mit einer Approval Rule und der Einstellung, dass Autorinnen und Autoren ihre
eigenen Merge Requests nicht freigeben dürfen.

Prüfen lässt sich die Regel mit einem direkten Push, der abgelehnt werden muss:

```bash
git switch main
git commit --allow-empty -m "chore: test branch protection"
git push origin main   # muss abgelehnt werden
git reset --hard HEAD~1
```

## Ausbauteil

Branch aufräumen:

```bash
git rebase -i main
# pick   -> Commit unverändert
# reword -> Nachricht ändern
# squash -> mit dem vorherigen Commit zusammenfassen, Nachrichten zusammenführen
# fixup  -> wie squash, eigene Nachricht verwerfen
# Zeilen umsortieren ändert die Reihenfolge der Commits
```

Verlorenen Stand zurückholen:

```bash
git reset --hard HEAD~3
git reflog                    # HEAD@{1} ist der Stand vor dem Reset
git reset --hard HEAD@{1}
```

Eigenen Tag setzen:

```bash
git tag -a v0.1.1-team.1 -m "v0.1.1 with team changes"
git push origin v0.1.1-team.1
```
