```bash
# Wie viele Anfragen wurden je Endpunkt bearbeitet?
cat logs.jsonl | jq -s 'map(select(.path)) | group_by(.path) | map({"path": .[0].path, "len": (. | length)}).[]'

# Welcher Endpunkt hat die höchste maximale Antwortzeit?
cat logs.jsonl | jq -s 'map(select(.path)) | group_by(.path) | map({"path": .[0].path, "max": (map(.duration_ms) | max)}) | max_by(.max)'

# Welche Anfragen wurden nicht mit einem 2xx-Statuscode beantwortet?
cat logs.jsonl | jq -s 'map(select(.status) | select(.status > 299 or .status < 200))'
```
