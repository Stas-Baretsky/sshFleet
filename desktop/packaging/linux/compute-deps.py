#!/usr/bin/env python3
"""
Считает полное замыкание зависимостей (Depends + Pre-Depends) пакетов
libgtk-3-0t64 и libwebkit2gtk-4.1-0, опираясь на то, какие именно
альтернативы уже установлены в текущей системе (dpkg -s). Так список
получается тем же, что реально нужен работающей системе, а не всеми
теоретически возможными альтернативами (в отличие от
`apt-cache depends --recurse`, который перечисляет вообще все варианты
для виртуальных пакетов и раздувает список в разы).

Запускать на машине, где приложение уже собиралось и запускалось
(т.е. GTK3/WebKit2GTK установлены). Результат - список имён пакетов,
по одному на строку, отсортированный.
"""

import re
import subprocess
import sys

ROOTS = ["libgtk-3-0t64", "libwebkit2gtk-4.1-0"]


def is_installed(pkg: str) -> bool:
    r = subprocess.run(["dpkg", "-s", pkg], capture_output=True, text=True)
    return r.returncode == 0 and "Status: install ok installed" in r.stdout


def build_provides_map() -> dict[str, str]:
    """
    Многие зависимости указывают виртуальный пакет (например
    dbus-session-bus), который сам по себе не ставится - его
    "предоставляет" (Provides:) какой-то другой, реально
    установленный пакет (например dbus-user-session). dpkg -s
    для виртуального имени ничего не найдёт, поэтому для таких
    имён нужен отдельный резолвинг через Provides.
    """
    r = subprocess.run(
        ["dpkg-query", "-W", "-f=${Package}\t${Provides}\n"],
        capture_output=True, text=True,
    )
    provides: dict[str, str] = {}
    for line in r.stdout.splitlines():
        if "\t" not in line:
            continue
        pkg, provided = line.split("\t", 1)
        for spec in provided.split(","):
            spec = spec.strip()
            if not spec:
                continue
            name = re.sub(r"\s*\(.*\)", "", spec).strip()
            if name:
                provides.setdefault(name, pkg)
    return provides


PROVIDES = build_provides_map()


def resolve(pkg: str) -> str | None:
    """Возвращает реальное (устанавливаемое) имя пакета для pkg,
    разворачивая виртуальные имена через Provides. None, если
    ни само имя, ни что-либо его предоставляющее не установлено."""
    if is_installed(pkg):
        return pkg
    real = PROVIDES.get(pkg)
    if real and is_installed(real):
        return real
    return None


def deps_of(pkg: str) -> list[list[str]]:
    r = subprocess.run(["dpkg-query", "-s", pkg], capture_output=True, text=True)
    if r.returncode != 0:
        return []

    fields = []
    for line in r.stdout.splitlines():
        if line.startswith("Depends:") or line.startswith("Pre-Depends:"):
            fields.append(line.split(":", 1)[1])

    groups = []
    for field in fields:
        for spec in field.split(","):
            spec = spec.strip()
            if not spec:
                continue
            alternatives = [a.strip() for a in spec.split("|")]
            names = [re.sub(r"\s*\(.*\)", "", a).split(":")[0].strip() for a in alternatives]
            groups.append(names)
    return groups


def main() -> None:
    seen: set[str] = set()
    queue = list(ROOTS)
    result: list[str] = []

    while queue:
        pkg = queue.pop()
        if pkg in seen:
            continue
        seen.add(pkg)

        real = resolve(pkg)
        if real is None:
            print(f"WARN: {pkg} not installed and nothing provides it, skipping", file=sys.stderr)
            continue

        result.append(real)

        for alternatives in deps_of(real):
            chosen = next((resolve(name) for name in alternatives if resolve(name)), None)
            if chosen and chosen not in seen:
                queue.append(chosen)

    for pkg in sorted(set(result)):
        print(pkg)


if __name__ == "__main__":
    main()
