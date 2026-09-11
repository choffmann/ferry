# API

Alle Antworten sind JSON. Fehler haben die Form

```json
{ "error": "departure is sold out", "request_id": "7c3f1a6e-9d24-4b58-8a01-5e2f0b7c4d19" }
```

Jede Antwort trägt den Kopf `X-Request-Id`. Wird er mitgeschickt, übernimmt der
Server ihn unverändert, sonst erzeugt er eine UUID.

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
| `201` | angelegt, die Buchung steht im Körper |
| `400` | Körper unlesbar oder Werte außerhalb des Erlaubten |
| `404` | die Abfahrt gibt es nicht |
| `409` | die Abfahrt ist ausgebucht (`departure is sold out`) oder der Buchungsschluss ist vorbei (`departure is closed for booking`) |

```json
{
  "id": "bk-000001",
  "departure_id": "FL-SO-2026-09-29T06",
  "passengers": 2,
  "status": "confirmed",
  "created_at": "2026-09-29T11:17:00Z"
}
```

## `GET /bookings/{id}`

Liefert eine Buchung, oder `404`.

## `GET /bookings/{id}/ticket`

Liefert den Bordausweis der Buchung als `text/plain`, oder `404`.

Der Text entsteht aus einer Vorlage im Asset-Verzeichnis, die die Anwendung beim
Start liest. Sie liegt unter `assets/tickets/`, `ASSETS_DIR` zeigt auf ein anderes
Verzeichnis. Fehlt die Vorlage, startet der Prozess nicht.

## Betriebsendpunkte

| Endpunkt | Antwort |
|---|---|
| `GET /healthz` | `200`, solange der Prozess läuft |
| `GET /readyz` | `200` wenn bereit, sonst `503` |
| `GET /version` | Version, Git-SHA und Build-Zeit |

```json
{ "version": "v0.2.0", "commit": "1a2b3c4", "build_time": "2026-10-09T08:00:00Z" }
```

Die drei Werte stammen aus dem Build und werden über `-ldflags` gesetzt. Ein Binary,
das ohne sie übersetzt wurde, liefert drei leere Felder und schreibt beim Start eine
Warnung ins Log.
