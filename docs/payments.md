# Zahlungsdienst

Seit v0.4.0 besteht das System aus zwei Diensten. Der zweite beantwortet eine
einzige Frage: ob eine Buchung über einen bestimmten Betrag abgerechnet werden
darf. Er hält keinen Zustand, kennt keine Datenbank und weiß nichts von
Abfahrten, Plätzen oder Fahrplänen.

Er ist auch kein weiterer Unterbefehl von `ferry`. Sein Quellcode liegt unter
`cmd/payments`, das Binary heißt `payments` und wird getrennt gebaut, gestartet
und ausgeliefert. Es ist ein eigener Prozess, der neben `ferry serve` und
`ferry worker` laufen muss.

Aufgerufen wird er vom Worker. Die API spricht nicht mit ihm. Der Weg einer
Buchung durch die drei Zustände steht in `api.md`.

## `POST /charge`

Die Anfrage nennt die Buchung und den Betrag in Cent.

```json
{ "booking_id": "bk-000001", "amount_cents": 2500 }
```

Beide Felder sind Pflicht. Unbekannte Felder werden abgewiesen.

Die Antwort hat zwei Formen. Eine genehmigte Zahlung trägt eine Referenz:

```json
{ "status": "approved", "reference": "pay-1c8f0d3a-5b47-4e92-9a61-0f3d7c2b8e54" }
```

Eine abgelehnte Zahlung trägt einen Grund:

```json
{ "status": "declined", "reason": "amount exceeds the card limit" }
```

Beide kommen mit `200`. Ein `400` gibt es nur für eine Anfrage, mit der der
Dienst nichts anfangen kann: unlesbarer Körper oder ein unbekanntes Feld
(`request body is not valid JSON`), fehlendes `booking_id` (`booking_id
missing`) oder ein `amount_cents` unter 1 (`amount_cents must be positive`). Der
Fehlerkörper hat dieselbe Form wie in der API, mit `error` und `request_id`.

### Warum eine Ablehnung `200` ist

Eine Ablehnung ist eine Auskunft über die Zahlung. Der Dienst hat die Anfrage
gelesen, hat entschieden und sagt, wie die Entscheidung ausgefallen ist. Ein
Timeout oder ein `500` ist keine solche Auskunft. Danach weiß der Aufrufer
nicht, ob abgerechnet wurde.

Der Worker trennt genau an dieser Linie. Er nimmt eine `200`-Antwort als
Ergebnis und schreibt sie in die Buchung. Ein Fehler, nach dem die Buchung offen
bleibt, ist für ihn jeder andere Statuscode, jeder Verbindungsfehler, eine
unlesbare Antwort und auch eine lesbare Antwort, deren `status` weder
`approved` noch `declined` lautet.

## Das Kartenlimit

`CardLimitCents` in `internal/payments/payments.go` steht auf 7500 Cent. Bis
einschließlich 7500 wird genehmigt, alles darüber abgelehnt. Ein anderes
Kriterium kennt der Dienst nicht.

Der Fahrpreis liegt bei 1250 Cent je Person, das ist
`domain.FareCentsPerPassenger`. Damit ergibt sich die Grenze im Fahrbetrieb:

| Personen | Betrag | Antwort |
|---|---|---|
| 6 | 7500 | `approved` |
| 7 | 8750 | `declined` |

Sechs Personen treffen das Limit genau und werden genehmigt. Ab sieben Personen
liegt der Betrag darüber, die Buchung wird abgelehnt. Weil `passengers` bis 9
reicht, ist die Grenze über die normale API erreichbar.

Das Limit ändert sich nicht mit der Umgebung, in der der Dienst läuft. Es steht
deshalb als Konstante im Code und ist nicht konfigurierbar.

## Betrieb

| Endpunkt | Antwort |
|---|---|
| `GET /healthz` | `200` mit `{"status":"ok"}` |
| `GET /readyz` | `200` mit `{"status":"ready"}` |
| `GET /version` | Version, Git-SHA und Build-Zeit |

`/readyz` antwortet `200`, solange der Prozess läuft. Der Dienst hat keine
Abhängigkeit, deren Ausfall er melden könnte: keine Datenbank, keine Assets,
keine Chaos-Schnittstelle. Die beiden Sonden unterscheiden sich hier nicht.

Jede Antwort trägt den Kopf `X-Request-Id` nach derselben Regel wie die API.

`PORT` ist die einzige Umgebungsvariable, Vorgabe ist `8080`. Weder
`DATABASE_URL` noch `ADMIN_TOKEN` werden gelesen. Die Werte hinter `/version`
kommen wie beim `ferry`-Binary über `-ldflags` hinein. Fehlen sie, schreibt der
Prozess beim Start eine Warnung ins Log.

## Wiederholte Abrechnung

Der Worker beansprucht eine Buchung, ruft `/charge` auf und schreibt das
Ergebnis zurück. Stirbt er zwischen dem Aufruf und dem Rückschreiben, ist die
Zahlung geschehen, die Buchung steht aber weiterhin auf `pending`. Nach einer
Minute gilt der Anspruch als verfallen und die Buchung wird erneut ausgegeben.
Sie wird dann ein zweites Mal abgerechnet.

Das ist hingenommen und kein Versehen. `Decide` entscheidet allein anhand von
`amount_cents`. Der Dienst führt kein Konto, merkt sich keine Buchungs-ID und
bucht nichts ab. Ein Zahlungsdienst, der Geld bewegt, würde `booking_id` als
Idempotenzschlüssel führen und den zweiten Aufruf auf die erste Zahlung
abbilden. Einen solchen Schlüssel gibt es hier nicht, ein zweiter Aufruf mit
derselben `booking_id` ist für den Dienst ein neuer Vorgang und liefert eine
neue Referenz.
