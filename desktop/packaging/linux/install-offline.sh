#!/usr/bin/env bash
#
# Офлайн-установщик SSH Fleet для Ubuntu/Debian.
# Ставится без интернета: все нужные .deb-пакеты (GTK3, WebKit2GTK
# и их зависимости) лежат рядом, в папке deps/.
#
# Собран под Ubuntu 24.04 (noble), x86-64. На другой версии Ubuntu
# или на Debian версии пакетов могут не совпасть с уже установленными
# в системе - тогда либо соберите пакет заново на подходящей машине
# (desktop/package-linux.sh), либо поставьте GTK3/WebKit2GTK из
# обычных репозиториев, если интернет всё же есть:
#   sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME=sshfleet
APP_TITLE="SSH Fleet"

echo "==> Проверка системных зависимостей (GTK3 / WebKit2GTK)..."

if dpkg -s libgtk-3-0t64 >/dev/null 2>&1 && dpkg -s libwebkit2gtk-4.1-0 >/dev/null 2>&1; then

    echo "    Уже установлены, пропускаю."

else

    echo "    Не найдены, устанавливаю из локального набора (deps/, ${SCRIPT_DIR})."
    echo "    Потребуются права администратора (sudo) и не потребуется интернет."

    # apt умеет ставить пакеты прямо из локальных .deb-файлов и сам
    # разрешает зависимости между ними - без интернета и без
    # dpkg-scanpackages/локального репозитория (который есть не на
    # каждой машине - это часть dpkg-dev, а не базовой системы).
    shopt -s nullglob
    deb_files=("$SCRIPT_DIR"/deps/*.deb)
    shopt -u nullglob

    if [[ ${#deb_files[@]} -eq 0 ]]; then
        echo "    ! Не найдены .deb-файлы в $SCRIPT_DIR/deps - установка невозможна." >&2
        exit 1
    fi

    sudo apt-get install -y "${deb_files[@]}"
fi

echo "==> Установка $APP_TITLE..."

INSTALL_DIR="$HOME/.local/share/$APP_NAME"
ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"
DESKTOP_DIR="$HOME/.local/share/applications"

mkdir -p "$INSTALL_DIR" "$ICON_DIR" "$DESKTOP_DIR"
install -m 755 "$SCRIPT_DIR/bin/$APP_NAME" "$INSTALL_DIR/$APP_NAME"
install -m 644 "$SCRIPT_DIR/icon/appicon.png" "$ICON_DIR/${APP_NAME}.png"

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
