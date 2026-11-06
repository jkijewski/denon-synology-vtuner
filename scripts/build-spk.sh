#!/bin/sh
# Builds dist/vtunerradio-<version>-<arch>.spk for each architecture in ARCHES.
# Usage: VERSION=1.0.0 BUILD=1 ARCHES="x86_64 armv8" scripts/build-spk.sh
# Optional: PROJECT_URL=https://... adds the project's web, support and help links to the
# package. It is not stored in the repository, so set it when you build a release.
set -eu

cd "$(dirname "$0")/.."
VERSION="${VERSION:-1.0.0}"
BUILD="${BUILD:-1}"
ARCHES="${ARCHES:-x86_64}"
PKG=vtunerradio
FULLVER="$VERSION-$(printf '%04d' "$BUILD")"
PROJECT_URL="${PROJECT_URL:-}"

# The CHANGELOG.md section for this version ("## [1.0.0] - 2026-11-06"), as one line for INFO.
changelog() {
    awk -v v="## [$VERSION]" '
        index($0, v) == 1 { on = 1; next }
        on && /^## / { exit }
        on && /^### / { next }
        on && NF { sub(/^- /, ""); gsub(/"/, "\x27"); out = out (out ? "<br>" : "") $0 }
        END { print out }' CHANGELOG.md 2>/dev/null
}

rm -rf build && mkdir -p build dist

for arch in $ARCHES; do
    case "$arch" in
        x86_64) goarch=amd64; goarm= ;;
        armv8)  goarch=arm64; goarm= ;;
        armv7)  goarch=arm;   goarm=7 ;;
        *) echo "unsupported arch: $arch" >&2; exit 1 ;;
    esac
    work="build/$arch"
    payload="$work/payload"
    mkdir -p "$payload/bin" "$payload/ui"

    echo "== building vtunerd for $arch"
    CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" GOARM="$goarm" \
        go build -trimpath -ldflags "-s -w -X main.version=$FULLVER" -o "$payload/bin/vtunerd" ./cmd/vtunerd

    cp -R package/ui/. "$payload/ui/"
    # Firewall service file for the default port; postinst rewrites it for the chosen port.
    mkdir -p "$payload/port_conf"
    printf '[%s]\ntitle="vTuner Radio"\ndesc="vTuner Radio"\nport_forward="no"\ndst.ports="8580/tcp"\n' "$PKG" > "$payload/port_conf/$PKG.sc"
    # DSM's web server reads the UI files, whatever the file modes are in this clone.
    chmod -R u=rwX,go=rX "$payload"
    chmod 755 "$payload/bin/vtunerd" "$payload/ui/index.cgi"
    tar -czf "$work/package.tgz" --owner=0 --group=0 -C "$payload" .

    cp -R package/scripts package/conf package/WIZARD_UIFILES "$work/"
    cp package/icons/PACKAGE_ICON.PNG package/icons/PACKAGE_ICON_256.PNG "$work/"
    # Package Center shows LICENSE in the install wizard.
    cp LICENSE "$work/LICENSE"
    # preupgrade compares this with the data on the NAS.
    echo "SCHEMA_VERSION=$(go run ./cmd/vtunerd schema-version)" >> "$work/scripts/common"
    chmod 755 "$work"/scripts/*
    chmod 644 "$work/scripts/common"

    cat > "$work/INFO" <<EOT
package="$PKG"
version="$FULLVER"
os_min_ver="7.0-40000"
displayname="vTuner Radio"
description="Self-hosted replacement for the vTuner internet radio directory. Serves station lists to older receivers (such as Denon) whose vTuner host names point at this NAS."
maintainer="denon-synology-vtuner contributors"
arch="$arch"
dsmuidir="ui"
dsmappname="SYNO.SDS._ThirdParty.App.$PKG"
ctl_stop="yes"
silent_upgrade="yes"
checksum="$(md5sum "$work/package.tgz" | cut -d' ' -f1)"
EOT
    if [ -n "$PROJECT_URL" ]; then
        printf 'maintainer_url="%s"\nsupport_url="%s/issues"\nhelpurl="%s/blob/main/docs/install-synology-package.md"\n' \
            "$PROJECT_URL" "$PROJECT_URL" "$PROJECT_URL" >> "$work/INFO"
    fi
    notes=$(changelog)
    [ -n "$notes" ] && printf 'changelog="%s"\n' "$notes" >> "$work/INFO"

    spk="dist/$PKG-$FULLVER-$arch.spk"
    tar -cf "$spk" --owner=0 --group=0 -C "$work" INFO LICENSE package.tgz scripts conf WIZARD_UIFILES PACKAGE_ICON.PNG PACKAGE_ICON_256.PNG
    echo "== built $spk"

    sh scripts/check-secrets.sh "$work"
done
