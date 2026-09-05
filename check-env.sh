#!/bin/sh
set -u

fail=0

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    green='\033[32m'
    red='\033[31m'
    reset='\033[0m'
else
    green=''
    red=''
    reset=''
fi

note() { printf '        %s\n' "$1"; }
ok()   { printf '%b%-7s%b %s\n' "$green" "[OK]" "$reset" "$1"; }
bad()  { printf '%b%-7s%b %s\n' "$red" "[FEHLT]" "$reset" "$1"; fail=1; }

check_cmd() {
    if command -v "$1" >/dev/null 2>&1; then
        # curl reports every protocol and library it was built with, on one line.
        ok "$1 ($($2 2>&1 | head -n 1 | cut -c1-48))"
    else
        bad "$1"
        note "$3"
    fi
}

printf 'ferry check-env, Stufe Block 1\n\n'

case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
        printf '%bABBRUCH%b\n' "$red" "$reset"
        printf 'Dieses Skript läuft in Git Bash oder MSYS. Im Kurs wird unter Windows\n'
        printf 'ausschließlich in WSL2 gearbeitet. Bitte WSL2 einrichten und dort erneut\n'
        printf 'starten, sonst gibt es Ärger mit Zeilenenden, Pfaden und Volume-Mounts.\n'
        exit 1
        ;;
esac

if grep -qi microsoft /proc/version 2>/dev/null; then
    note "Umgebung: WSL2, richtig so"
fi

check_cmd git "git --version" "Installation: https://git-scm.com/downloads"
check_cmd ssh "ssh -V" "Teil von OpenSSH, unter Debian/Ubuntu: apt install openssh-client"
check_cmd curl "curl --version" "Unter Debian/Ubuntu: apt install curl, unter macOS vorinstalliert"
check_cmd jq "jq --version" "Unter Debian/Ubuntu: apt install jq, unter macOS: brew install jq"
check_cmd go "go version" "Installation: https://go.dev/dl/, mindestens Go 1.26"
check_cmd docker "docker --version" "Docker Engine oder Docker Desktop, siehe Setup-Doku"

if command -v docker >/dev/null 2>&1; then
    if docker info >/dev/null 2>&1; then
        ok "docker daemon erreichbar"
    else
        bad "docker daemon nicht erreichbar"
        note "Docker Desktop starten, oder unter Linux: sudo systemctl start docker"
        note "Ohne sudo arbeiten: sudo usermod -aG docker \$USER, danach neu anmelden"
    fi
fi

printf '\n'
if [ "$fail" -eq 0 ]; then
    printf '%bAlles da.%b\n' "$green" "$reset"
else
    printf '%bEs fehlt etwas.%b Bitte nachinstallieren.\n' "$red" "$reset"
fi
exit "$fail"
