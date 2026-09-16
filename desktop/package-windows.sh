#!/usr/bin/env bash
#
# Кросс-компилирует SSH Fleet под Windows (amd64) и собирает офлайн-пакет:
# .exe + офлайн-инсталлятор Microsoft Edge WebView2 Runtime (без него
# приложение на Windows не запустится - это движок отрисовки интерфейса)
# + install.bat, который ставит и то, и другое без интернета.
#
# Запускать на машине с интернетом (для однократной загрузки WebView2
# Runtime, ~200 МБ - дальше он кешируется и повторно не скачивается).
# Сама кросс-компиляция интернета не требует и cgo/mingw не использует -
# у Wails v2 для Windows чистый Go-бэкенд (WebView2 через syscall/COM).
#
# ВНИМАНИЕ: этот .exe не был протестирован на реальной Windows - здесь
# нет такой машины. Проверьте перед раздачей коллегам.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DESKTOP_DIR="$REPO_ROOT/desktop"
DIST_DIR="$REPO_ROOT/dist/windows"
CACHE_DIR="$REPO_ROOT/dist/.cache"

WV2_URL="https://go.microsoft.com/fwlink/p/?LinkId=2124701"
WV2_FILE="MicrosoftEdgeWebView2RuntimeInstallerX64.exe"

echo "==> Генерация ресурсов .exe (иконка, без консольного окна)..."
mkdir -p "$CACHE_DIR"
if ! command -v go-winres >/dev/null 2>&1; then
    GOBIN="$CACHE_DIR/gobin" go install github.com/tc-hib/go-winres@latest
    WINRES="$CACHE_DIR/gobin/go-winres"
else
    WINRES="go-winres"
fi

(cd "$DESKTOP_DIR" && "$WINRES" simply --icon build/appicon.ico --product-name "SSH Fleet" --file-description "SSH Fleet")

echo "==> Кросс-компиляция под Windows amd64..."
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"
cd "$REPO_ROOT"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
    -tags production \
    -ldflags "-H=windowsgui -s -w" \
    -o "$DIST_DIR/sshfleet.exe" \
    ./desktop

rm -f "$DESKTOP_DIR"/rsrc_windows_*.syso

echo "==> Проверка/загрузка WebView2 Runtime Installer (офлайн, ~200 МБ, кешируется)..."
if [[ ! -f "$CACHE_DIR/$WV2_FILE" ]]; then
    curl -sL -o "$CACHE_DIR/$WV2_FILE" "$WV2_URL"
fi
cp "$CACHE_DIR/$WV2_FILE" "$DIST_DIR/$WV2_FILE"

cp "$DESKTOP_DIR/packaging/windows/install.bat" "$DIST_DIR/install.bat"
python3 - "$DIST_DIR/install.bat" <<'PYEOF'
import sys
path = sys.argv[1]
data = open(path, "rb").read().replace(b"\r\n", b"\n").replace(b"\n", b"\r\n")
open(path, "wb").write(data)
PYEOF

cd "$REPO_ROOT/dist"
zip_name="sshfleet-windows-amd64-offline.zip"
rm -f "$zip_name"
zip -rq "$zip_name" windows
echo "==> Готово: $REPO_ROOT/dist/$zip_name ($(du -h "$zip_name" | cut -f1))"
echo "    На целевой машине: распаковать и запустить windows\\install.bat"
