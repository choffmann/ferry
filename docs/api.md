# API

Alle Antworten sind JSON. Fehler haben die Form

```json
{ "error": "departure is sold out", "request_id": "7c3f1a6e-9d24-4b58-8a01-5e2f0b7c4d19" }
```

Jede Antwort trägt den Kopf `X-Request-Id`. Wird er mitgeschickt, übernimmt der
Server ihn unverändert, sonst erzeugt er eine UUID.

Verbindungen, Abfahrten und Buchungen liegen seit v0.3.0 in Postgres. Eine
angelegte Buchung überlebt damit einen Neustart. Schema und Fahrplan beschreibt
`database.md`.

## `GET /connections`

Liefert alle Verbindungen.

```json
[
  {
    "id": "FL-SO",
    "from": { "code": "FL", "name": "Flensburg" },
    "to": { "code": "SO", "name": "Sønderborg" },
    "duration_minutes": 95
  }
]
```

## `GET /connections/{id}/departures`

Liefert die Abfahrten einer Verbindung, aufsteigend nach Zeit. Der Fahrplan
umfasst sieben Tage ab dem aktuellen Tag. Abfahrten, deren Buchungsschluss
vorbei ist, stehen nicht in der Liste.

Die Abfahrtszeiten sind Ortszeiten der Häfen, der Fahrplan steht in der Zeitzone
`Europe/Berlin`. Eine Abfahrt um 06:00 bleibt eine Abfahrt um 06:00, auch in der
Woche, in der die Sommerzeit endet. Findet der Prozess die Zeitzone nicht, startet
er nicht.

Parameter `from`, optional: Zeitstempel nach RFC3339. Abfahrten davor werden
weggelassen. Ein unlesbarer Wert ergibt `400`. Ein Wert in der Vergangenheit
holt keine abgefahrenen Verbindungen zurück.

```json
[
  {
    "id": "FL-SO-2026-09-29T06",
    "connection_id": "FL-SO",
    "departs_at": "2026-09-29T06:00:00+02:00",
    "capacity": 40,
    "booked": 0
  }
]
```

## Der Zustand einer Buchung

Seit v0.4.0 wird eine Buchung nicht mehr im Zuge der Anfrage abgerechnet. Sie
durchläuft drei Zustände.

| Status | Bedeutung |
|---|---|
| `pending` | angelegt, die Zahlung steht noch aus |
| `confirmed` | die Zahlung wurde genehmigt |
| `failed` | die Zahlung wurde abgelehnt |

`POST /bookings` legt die Buchung als `pending` an und antwortet sofort. Den Weg
weiter geht der Worker, ein zweiter Prozess desselben Binaries, gestartet mit
`ferry worker`. Er nimmt sich die offenen Buchungen vor, fragt für jede den
Zahlungsdienst und schreibt das Ergebnis zurück. Aus `pending` wird damit
`confirmed` oder `failed`. Den Zahlungsdienst beschreibt `payments.md`.

Die Plätze sind schon mit dem `201` reserviert. `booked` der Abfahrt steigt
also, solange die Buchung noch offen ist. Eine ausgebuchte Abfahrt bleibt
ausgebucht, während der Worker die Zahlungen klärt. Erst mit `failed` werden die
Plätze wieder freigegeben. Eine bestätigte Buchung behält sie.

Antwortet der Zahlungsdienst nicht, bleibt die Buchung `pending`. Ein Ausfall
des Dienstes erzeugt keine fehlgeschlagenen Buchungen, er schiebt die
Entscheidung nur auf. Läuft gar kein Worker, bleiben die Buchungen ebenfalls
offen.

## `POST /bookings`

Legt eine Buchung an.

```json
{ "departure_id": "FL-SO-2026-09-29T06", "passengers": 2 }
```

`passengers` liegt zwischen 1 und 9.

Der Buchungsschluss liegt 15 Minuten vor der Abfahrt. Danach nimmt die Abfahrt
keine Buchung mehr an, auch nicht mit eingeschalteter Überbuchung.

| Status | Bedeutung |
|---|---|
| `201` | angelegt, die Buchung steht als `pending` im Körper |
| `400` | Körper unlesbar oder Werte außerhalb des Erlaubten |
| `404` | die Abfahrt gibt es nicht |
| `409` | die Abfahrt ist ausgebucht (`departure is sold out`) oder der Buchungsschluss ist vorbei (`departure is closed for booking`) |

```json
{
  "id": "bk-000001",
  "departure_id": "FL-SO-2026-09-29T06",
  "passengers": 2,
  "amount_cents": 2500,
  "status": "pending",
  "created_at": "2026-09-29T11:17:00Z"
}
```

`amount_cents` ist der Preis der Buchung, 1250 Cent je Person. Er wird beim
Anlegen berechnet und ändert sich danach nicht. Der Worker nennt ihn dem
Zahlungsdienst.

## `GET /bookings/{id}`

Liefert eine Buchung, oder `404`.

Die Antwort hat dieselbe Form wie die von `POST /bookings`. Zwei Felder kommen
erst mit der Entscheidung des Workers hinzu und fehlen vorher:

| Feld | Wann |
|---|---|
| `payment_ref` | bei `confirmed`, die Referenz der Zahlung |
| `failure_reason` | bei `failed`, der Grund der Ablehnung |

```json
{
  "id": "bk-000001",
  "departure_id": "FL-SO-2026-09-29T06",
  "passengers": 2,
  "amount_cents": 2500,
  "status": "confirmed",
  "payment_ref": "pay-1c8f0d3a-5b47-4e92-9a61-0f3d7c2b8e54",
  "created_at": "2026-09-29T11:17:00Z"
}
```

## `GET /bookings/{id}/ticket`

Liefert den Bordausweis der Buchung als `text/plain`, oder `404`.

Einen Bordausweis gibt es nur für eine bestätigte Buchung. Steht die Buchung auf
`pending` oder `failed`, antwortet der Endpunkt mit `409` und
`boarding pass is available only for a confirmed booking`. Bei `pending` lohnt
sich ein zweiter Versuch, sobald der Worker die Buchung abgerechnet hat.

Der Text entsteht aus einer Vorlage im Asset-Verzeichnis, die die Anwendung beim
Start liest. Sie liegt unter `assets/tickets/`, `ASSETS_DIR` zeigt auf ein anderes
Verzeichnis. Fehlt die Vorlage, startet der Prozess nicht.

## Betriebsendpunkte

| Endpunkt | Antwort |
|---|---|
| `GET /healthz` | `200`, solange der Prozess läuft |
| `GET /readyz` | `200` wenn bereit, sonst `503` |
| `GET /version` | Version, Git-SHA und Build-Zeit |

`/readyz` prüft zweierlei: den Bereitschaftsschalter der Chaos-Schnittstelle und
die Verbindung zur Datenbank. Antwortet die Datenbank nicht, steht im Feld
`status` der Wert `database unreachable`. `/healthz` bleibt davon unberührt, es
sagt nur, dass der Prozess lebt.

```json
{ "version": "v0.2.0", "commit": "1a2b3c4", "build_time": "2026-10-09T08:00:00Z" }
```

Die drei Werte stammen aus dem Build und werden über `-ldflags` gesetzt. Ein Binary,
das ohne sie übersetzt wurde, liefert drei leere Felder und schreibt beim Start eine
Warnung ins Log.
