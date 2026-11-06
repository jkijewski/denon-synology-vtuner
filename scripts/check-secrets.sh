#!/bin/sh
# Fails if files contain anything that looks like a real address, MAC, key or password.
# Only the placeholder values used in the docs are allowed.
# Usage: scripts/check-secrets.sh [dir ...]   (default: the git-tracked and untracked files of the repo)
# Host names and user names can't be recognised by their shape. List your own, one per line,
# in .secrets-denylist, which git ignores, and any file holding one of them fails.
set -eu
cd "$(dirname "$0")/.."

# Besides the placeholders, this allows special-use addresses that the relay's address check names and tests.
ALLOWED_IPS='^(127\.0\.0\.1|0\.0\.0\.0|255\.255\.255\.0|8\.8\.8\.8|192\.168\.1\.(1|10|20|50|51|99)|10\.0\.0\.1|172\.16\.0\.1|100\.64\.0\.[01]|169\.254\.169\.254|224\.0\.0\.1|192\.0\.0\.0|192\.0\.2\.0|198\.18\.0\.0|198\.51\.100\.0|203\.0\.113\.0|240\.0\.0\.0)$'
ALLOWED_MACS='^(00[:-]?11[:-]?22[:-]?33[:-]?44[:-]?(55|66)|AA-BB-CC-00-00-0[1-9])$'

if [ $# -eq 0 ]; then
    files=$(git ls-files --cached --others --exclude-standard | grep -v '\.png$\|\.PNG$' || true)
else
    files=$(find "$@" -type f ! -name '*.png' ! -name '*.PNG' ! -name '*.tgz' ! -name vtunerd)
fi

denylist=.secrets-denylist
fail=0
for f in $files; do
    [ -f "$f" ] || continue
    for ip in $(grep -aoE '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' "$f" | sort -u); do
        echo "$ip" | grep -Eq "$ALLOWED_IPS" || { echo "$f: address $ip is not a placeholder"; fail=1; }
    done
    for mac in $(grep -aoiE '\b([0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b' "$f" | sort -u); do
        echo "$mac" | grep -Eqi "$ALLOWED_MACS" || { echo "$f: MAC $mac is not a placeholder"; fail=1; }
    done
    if grep -aqE 'BEGIN [A-Z ]*PRIVATE KEY|(password|passwd|secret|api[_-]?key)[[:space:]]*[:=][[:space:]]*["'\'']?[A-Za-z0-9/+]{8,}' "$f"; then
        echo "$f: looks like a key or password"; fail=1
    fi
    if [ -s "$denylist" ] && grep -aqiFf "$denylist" "$f"; then
        echo "$f: holds a name from $denylist"; fail=1
    fi
done
case "$files" in *admin.token*|*.env\ *|*.env) echo "token or .env file must not be committed"; fail=1 ;; esac

if [ $fail -ne 0 ]; then
    echo "secrets check: FAILED"
    exit 1
fi
echo "secrets check: OK"
