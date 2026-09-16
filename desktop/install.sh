#!/usr/bin/env bash
#
# Собирает SSH Fleet и устанавливает его как обычное приложение
# для текущего пользователя (без sudo): бинарник, иконка и ярлык
# в меню приложений.
#
# Требует: libgtk-3-dev, libwebkit2gtk-4.1-dev (см. README.md).
# Если в системе установлен webkit2gtk 4.0, а не 4.1 - уберите
# ",webkit2_41" из BUILD_TAGS ниже.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

APP_NAME=sshfleet
APP_TITLE="SSH Fleet"
BUILD_TAGS="production,webkit2_41"

INSTALL_DIR="$HOME/.local/share/$APP_NAME"
ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"
DESKTOP_DIR="$HOME/.local/share/applications"

echo "==> Сборка ($BUILD_TAGS)..."
cd "$REPO_ROOT"
tmp_bin="$(mktemp)"
go build -tags "$BUILD_TAGS" -o "$tmp_bin" ./desktop

echo "==> Установка в $INSTALL_DIR"
mkdir -p "$INSTALL_DIR" "$ICON_DIR" "$DESKTOP_DIR"
install -m 755 "$tmp_bin" "$INSTALL_DIR/$APP_NAME"
rm -f "$tmp_bin"

install -m 644 "$REPO_ROOT/desktop/build/appicon.png" "$ICON_DIR/${APP_NAME}.png"

cat > "$DESKTOP_DIR/${APP_NAME}.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=$APP_TITLE
Comment=Массовое выполнение команд на сетевых устройствах по SSH
Exec=$INSTALL_DIR/$APP_NAME
Icon=$APP_NAME
Terminal=false
Categories=Network;
StartupWMClass=$APP_NAME
EOF
chmod +x "$DESKTOP_DIR/${APP_NAME}.desktop"

command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$DESKTOP_DIR" >/dev/null 2>&1 || true
command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t "$HOME/.local/share/icons/hicolor" >/dev/null 2>&1 || true

echo "==> Готово."
echo "    Запустить: $INSTALL_DIR/$APP_NAME"
echo "    Или через меню приложений: «$APP_TITLE»"
