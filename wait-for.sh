#!/usr/bin/env bash
set -uo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <url> <timeout-seconds>" >&2
  exit 2
fi

url=$1
timeout=$2
deadline=$((SECONDS + timeout))

while true; do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 2 "$url")
  if [[ $code == 2?? ]]; then
    exit 0
  fi
  if (( SECONDS >= deadline )); then
    echo "timeout: $url not ready after ${timeout}s" >&2
    exit 1
  fi
  sleep 1
done
