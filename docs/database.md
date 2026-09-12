# Datenbank

Seit v0.3.0 liegt der Zustand in Postgres. Getestet wird gegen Postgres 17. Die
Anwendung braucht `DATABASE_URL`. Ohne die Variable starten `serve`, `worker`,
`migrate` und `seed` nicht.

## Schema

Vier Tabellen und eine Sequenz.

| Tabelle | Inhalt |
|---|---|
| `connections` | die vier Verbindungen mit Häfen und Fahrtdauer |
| `departures` | Abfahrten mit `departs_at`, `capacity` und `booked` |
| `bookings` | Buchungen mit Personenzahl, Betrag, Status und Zeitpunkten |
| `schema_migrations` | welche Migration wann gelaufen ist |

Die Buchungs-ID entsteht aus der Sequenz `booking_ids` und behält das Format
`bk-000001`. `departs_at` ist `TIMESTAMPTZ`. Der Fahrplan steht auf der Ortszeit
von `Europe/Berlin`, die Anwendung rechnet gelesene Zeitpunkte dorthin zurück.

Die Obergrenze aus `capacity` steht bewusst nicht als Bedingung in der Datenbank.
Die Regel liegt im Code, weil der Überbuchungs-Schalter der Chaos-Schnittstelle
sie aufheben können muss.

### Die Spalten des Buchungslebenszyklus

v0.4.0 hat `bookings` um fünf Spalten erweitert. Sie tragen den Weg von
`pending` nach `confirmed` oder `failed`, den `api.md` beschreibt.

| Spalte | Typ | Inhalt |
|---|---|---|
| `amount_cents` | `INTEGER NOT NULL DEFAULT 0` | der Preis der Buchung in Cent |
| `claimed_at` | `TIMESTAMPTZ` | wann ein Worker die Buchung beansprucht hat |
| `settled_at` | `TIMESTAMPTZ` | wann das Ergebnis zurückgeschrieben wurde |
| `payment_ref` | `TEXT` | die Referenz einer genehmigten Zahlung |
| `failure_reason` | `TEXT` | der Grund einer abgelehnten Zahlung |

Die vier nachgetragenen Zeit- und Textspalten sind `NULL`-bar, weil sie für eine
offene Buchung noch keinen Wert haben. `amount_cents` hat eine Vorgabe, damit
die Migration auf einer Datenbank mit bestehenden Buchungen durchläuft.
`payment_ref` und `failure_reason` schließen sich gegenseitig aus, gesetzt wird
immer nur die eine, die zum Ausgang passt.

### Wie der Worker eine Buchung beansprucht

`Claim` in `internal/store/queue.go` sucht offene Buchungen mit
`FOR UPDATE SKIP LOCKED` und setzt in derselben Anweisung `claimed_at`. Die
Sperre hält zwei gleichzeitig laufende Worker auseinander: was der eine gerade
liest, überspringt der andere und nimmt sich die nächsten Zeilen.

Das Beanspruchen ist eine einzige Anweisung, damit über den HTTP-Aufruf beim
Zahlungsdienst hinweg keine Transaktion offen steht. Die Sperre endet also
sofort, `claimed_at` bleibt als Merkmal zurück. Eine Buchung mit gesetztem
`claimed_at` wird nicht noch einmal ausgegeben, solange der Anspruch gilt.

Stirbt ein Worker zwischen dem Aufruf und dem Rückschreiben, bliebe seine
Buchung damit für immer liegen. Deshalb gilt ein Anspruch nach einer Minute als
verfallen, die Buchung wird dann erneut ausgegeben. Das ist der Fall, in dem
eine Buchung ein zweites Mal abgerechnet wird, `payments.md` geht darauf ein.
Antwortet der Zahlungsdienst nicht, wartet der Worker die Minute nicht ab: er
setzt `claimed_at` selbst wieder auf `NULL`.

`Settle` schreibt Status, Referenz oder Grund und `settled_at` in einer
Transaktion. Die Bedingung `status = 'pending'` im `UPDATE` sorgt dafür, dass
eine zweite Abrechnung ins Leere läuft. Ohne sie würden bei `failed` die Plätze
zweimal freigegeben.

Für die Suche nach offenen Buchungen legt die Migration den Teilindex
`bookings_pending` auf `created_at` an, der nur die Zeilen mit
`status = 'pending'` führt.

## Migrationen

Die SQL-Dateien liegen in `migrations/` und wandern beim Übersetzen in das
Binary. Es ist also nichts zu kopieren, das Artefakt bleibt Binary und `assets/`.

```sh
./ferry migrate
```

Der Befehl nimmt eine Sperre und legt `schema_migrations` an, falls sie fehlt.
Danach arbeitet er die Dateien in der Reihenfolge ihrer Namen ab. Jede läuft in
einer eigenen Transaktion, zusammen mit ihrem Eintrag in `schema_migrations`. Was
schon eingetragen ist, wird übersprungen.

Zweimal laufen lassen ist deshalb unproblematisch und meldet beim zweiten Mal,
dass nichts zu tun war.

Seit v0.4.0 gibt es zwei Dateien: `0001_schema.sql` legt die Tabellen und die
Sequenz an, `0002_booking_lifecycle.sql` trägt die fünf Spalten und den
Teilindex nach. Die Reihenfolge kommt allein aus den Dateinamen, die der Läufer
alphabetisch sortiert. Eine Liste im Code gibt es nicht. Der Dateiname ist
zugleich der Eintrag in `schema_migrations`, samt Endung `.sql`.

## Seed

```sh
./ferry seed
```

Schreibt die Verbindungen und den Fahrplan der kommenden sieben Tage. Jede Zeile
wird mit `ON CONFLICT DO NOTHING` eingefügt, belegte Plätze bleiben also
unangetastet. Weil das Fenster beim heutigen Datum beginnt, schiebt ein erneuter
Lauf den Fahrplan nach vorn und ergänzt, was fehlt.

Der Fahrplan steht nicht in einer Migration, weil er aus dem Datum entsteht. Eine
Migration beschreibt die Form, kein Inventar.

## Tests

Die Tests gegen die Datenbank lesen ihre Adresse aus `TEST_DATABASE_URL`. Ist die
Variable nicht gesetzt, überspringen sie sich, `go test ./...` läuft dann ohne
Datenbank durch.

```sh
TEST_DATABASE_URL='postgres://ferry:ferry@localhost:5432/ferry?sslmode=disable' go test ./...
```

Jeder Fall legt sich ein eigenes Schema an und räumt es hinterher wieder ab. Was
sonst in der angegebenen Datenbank liegt, fassen die Tests nicht an.
