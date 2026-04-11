#!/bin/bash
set -e
cd "$(dirname "$0")/.."

VERSION="${VERSION:-0.1.0}"
DEVICES="${DEVICE:-r1s}"
ALL_DEVICES="r1s r1q r1z c1 q1 q1n p1"

if [ "$DEVICES" = "all" ]; then
    DEVICES="$ALL_DEVICES"
fi

echo "=== MYT Panel Build v${VERSION} ==="

# 1. build frontend
echo "[Frontend] Building..."
cd frontend && npm install --silent 2>&1 | tail -3
npm run build 2>&1 | tail -5
cd ..
echo "[Frontend] OK"
echo ""

export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH

# 2. build each device
for DEV in $DEVICES; do
    LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Device=${DEV}"
    RELEASE_DIR="release/${DEV}/v${VERSION}"

    echo "[${DEV}] Compiling..."

    if command -v garble &> /dev/null; then
        GOOS=linux GOARCH=arm64 CGO_ENABLED=0 garble -literals -tiny build -trimpath -ldflags "${LDFLAGS}" -o myt-panel .
    else
        GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "${LDFLAGS}" -o myt-panel .
    fi

    SHA=$(sha256sum myt-panel | awk '{print $1}')
    echo "${SHA}" > myt-panel.sha256

    if command -v upx &> /dev/null; then
        upx --best --lzma myt-panel -q
        SHA=$(sha256sum myt-panel | awk '{print $1}')
        echo "${SHA}" > myt-panel.sha256
    fi

    mkdir -p "${RELEASE_DIR}/deploy"
    mv -f myt-panel myt-panel.sha256 "${RELEASE_DIR}/"
    echo "v${VERSION}" > "${RELEASE_DIR}/VERSION"
    cp -f deploy/alpine-openrc "${RELEASE_DIR}/deploy/"
    cp -f deploy/debian-systemd.service "${RELEASE_DIR}/deploy/"
    cp -f deploy/install-alpine.sh "${RELEASE_DIR}/deploy/"
    cp -f deploy/install-debian.sh "${RELEASE_DIR}/deploy/"
    cp -f deploy/README.txt "${RELEASE_DIR}/"

    echo "[${DEV}] OK  ${SHA}"
    echo ""
done

echo "=== Build complete! v${VERSION} ==="
echo ""
for DEV in $DEVICES; do
    if [ -f "release/${DEV}/v${VERSION}/myt-panel" ]; then
        ls -lh "release/${DEV}/v${VERSION}/myt-panel"
    fi
done
