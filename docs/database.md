# Datenbank

Seit v0.3.0 liegt der Zustand in Postgres. Getestet wird gegen Postgres 17. Die
Anwendung braucht `DATABASE_URL`, ohne die Variable startet kein Unterbefehl.

## Schema

Vier Tabellen und eine Sequenz.

| Tabelle | Inhalt |
|---|---|
| `connections` | die vier Verbindungen mit Häfen und Fahrtdauer |
| `departures` | Abfahrten mit `departs_at`, `capacity` und `booked` |
| `bookings` | Buchungen mit Personenzahl, Status und Zeitpunkt |
| `schema_migrations` | welche Migration wann gelaufen ist |

Die Buchungs-ID entsteht aus der Sequenz `booking_ids` und behält das Format
`bk-000001`. `departs_at` ist `TIMESTAMPTZ`. Der Fahrplan steht auf der Ortszeit
von `Europe/Berlin`, die Anwendung rechnet gelesene Zeitpunkte dorthin zurück.

Die Obergrenze aus `capacity` steht bewusst nicht als Bedingung in der Datenbank.
Die Regel liegt im Code, weil der Überbuchungs-Schalter der Chaos-Schnittstelle
sie aufheben können muss.

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
