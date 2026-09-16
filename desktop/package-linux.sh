#!/usr/bin/env bash
#
# Собирает исчерпывающий офлайн-пакет SSH Fleet для Linux (Ubuntu/Debian,
# x86-64): бинарник + иконка + все .deb-зависимости GTK3/WebKit2GTK,
# которых может не быть на машине без интернета.
#
# Запускать на машине С интернетом и apt (например на этой же, где
# разрабатывается приложение). Результат - dist/sshfleet-linux-amd64-offline.tar.gz,
# который можно перенести на офлайн-машину (флешкой и т.п.) и распаковать.
#
# Список пакетов в deps.txt - это полное замыкание зависимостей
# libgtk-3-0t64 и libwebkit2gtk-4.1-0, посчитанное по уже установленным
# на этой машине пакетам (packaging/linux/deps.txt). Если нужно
# пересчитать список заново (например после обновления системы),
# передайте --recompute-deps.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DESKTOP_DIR="$REPO_ROOT/desktop"
DIST_DIR="$REPO_ROOT/dist/linux"
BUILD_TAGS="production,webkit2_41"

RECOMPUTE_DEPS=0
[[ "${1:-}" == "--recompute-deps" ]] && RECOMPUTE_DEPS=1

echo "==> Сборка бинарника ($BUILD_TAGS)..."
cd "$REPO_ROOT"
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR/bin" "$DIST_DIR/icon" "$DIST_DIR/deps"
go build -tags "$BUILD_TAGS" -o "$DIST_DIR/bin/sshfleet" ./desktop

cp "$DESKTOP_DIR/build/appicon.png" "$DIST_DIR/icon/appicon.png"
cp "$DESKTOP_DIR/packaging/linux/install-offline.sh" "$DIST_DIR/install.sh"
chmod +x "$DIST_DIR/install.sh"

DEPS_FILE="$DESKTOP_DIR/packaging/linux/deps.txt"

if [[ "$RECOMPUTE_DEPS" == "1" ]]; then
    echo "==> Пересчитываю список зависимостей..."
    python3 "$DESKTOP_DIR/packaging/linux/compute-deps.py" > "$DEPS_FILE"
fi

echo "==> Скачиваю $(wc -l < "$DEPS_FILE") .deb-пакетов..."
cd "$DIST_DIR/deps"
apt-get download $(cat "$DEPS_FILE") 2>&1 | tail -20 || true

echo "==> Проверяю полноту..."
missing=0
while read -r pkg; do
    if ! compgen -G "${pkg}_*.deb" > /dev/null; then
        echo "    ! не скачался: $pkg (попробуйте вручную: apt-get download $pkg)"
        missing=1
    fi
done < "$DEPS_FILE"
[[ "$missing" == "1" ]] && echo "    Некоторые пакеты не скачались - см. выше, часто помогает повторный запуск (зеркало могло не успеть обновиться)."

cd "$REPO_ROOT/dist"
tar_name="sshfleet-linux-amd64-offline.tar.gz"
tar czf "$tar_name" -C "$REPO_ROOT/dist" linux
echo "==> Готово: $REPO_ROOT/dist/$tar_name ($(du -h "$tar_name" | cut -f1))"
echo "    На целевой машине: распаковать и запустить linux/install.sh"
