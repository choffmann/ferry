# Chaos-Schnittstelle

Über `/admin/chaos` lassen sich Fehlerbilder gezielt herbeiführen. Alle drei
Methoden verlangen den Kopf `Authorization: Bearer <ADMIN_TOKEN>` und antworten
sonst mit `401`.

| Methode | Wirkung |
|---|---|
| `GET` | liefert den aktuellen Zustand |
| `POST` | setzt Schalter, die der Körper nennt |
| `DELETE` | setzt alle Schalter auf ihren Ausgangswert |

Der Zustand ist additiv und teilweise setzbar: ein `POST` verändert nur die
Schalter, die im Körper vorkommen. Was fehlt, bleibt unverändert.

Der Zustand liegt im Arbeitsspeicher der Instanz, die die Anfrage beantwortet hat,
und überlebt keinen Neustart.

Die Endpunkte unter `/admin/` sind von den Schaltern ausgenommen. Sonst könnte man
sich mit einer hohen Latenz oder einer Fehlerrate von 1.0 selbst aus dem
Ausschalter aussperren. `/healthz` ist ebenfalls ausgenommen.

## Schalter

### `http.latency_ms`

Ganzzahl, Millisekunden. Verzögert jede Antwort außerhalb von `/admin/` und
`/healthz`.

```json
{ "http": { "latency_ms": 500 } }
```

### `http.error_rate` und `http.status`

`error_rate` ist ein Anteil zwischen 0 und 1 und bestimmt, welcher Teil der
Anfragen mit einem Fehler beantwortet wird. `status` legt den Statuscode fest,
Vorgabe ist `500`. Werte außerhalb von 400 bis 599 werden auf `500` zurückgesetzt.

```json
{ "http": { "error_rate": 0.25, "status": 503 } }
```

### `readiness`

`"ok"` oder `"fail"`. Bei `"fail"` antwortet `/readyz` mit `503`, während
`/healthz` weiterhin `200` liefert.

```json
{ "readiness": "fail" }
```

### `resources.memory_leak_mb_per_min`

Ganzzahl. Der Prozess belegt zusätzlichen Speicher in dieser Geschwindigkeit und
gibt ihn nicht zurück. `0` oder ein `DELETE` gibt das Belegte wieder frei.

```json
{ "resources": { "memory_leak_mb_per_min": 32 } }
```

### `booking.allow_overbooking`

Boolescher Wert. Ist er `true`, entfällt die Kapazitätsprüfung bei
`POST /bookings` vollständig: die Buchung gelingt unabhängig davon, wie viele
Plätze die betroffene Abfahrt noch hat, und `booked` kann `capacity`
übersteigen. Der Buchungsschluss bleibt davon unberührt.

```json
{ "booking": { "allow_overbooking": true } }
```

## Zustand vollständig

```json
{
  "http": { "latency_ms": 0, "error_rate": 0, "status": 0 },
  "readiness": "ok",
  "resources": { "memory_leak_mb_per_min": 0 },
  "booking": { "allow_overbooking": false }
}
```
