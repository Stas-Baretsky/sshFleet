@echo off
setlocal enabledelayedexpansion

set SCRIPT_DIR=%~dp0
set APP_TITLE=SSH Fleet
set INSTALL_DIR=%LOCALAPPDATA%\SSHFleet

echo ==^> Проверка Microsoft Edge WebView2 Runtime...

set WV2_FOUND=0
reg query "HKLM\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" /v pv >nul 2>&1
if !errorlevel! equ 0 set WV2_FOUND=1
reg query "HKCU\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" /v pv >nul 2>&1
if !errorlevel! equ 0 set WV2_FOUND=1

if "!WV2_FOUND!"=="1" (
    echo     Уже установлен, пропускаю.
) else (
    echo     Не найден, устанавливаю из локального пакета ^(понадобятся права администратора^)...
    "%SCRIPT_DIR%MicrosoftEdgeWebView2RuntimeInstallerX64.exe" /silent /install
    if !errorlevel! neq 0 (
        echo     ! Установка WebView2 завершилась с кодом !errorlevel!. Попробуйте запустить
        echo       "%SCRIPT_DIR%MicrosoftEdgeWebView2RuntimeInstallerX64.exe" вручную.
    )
)

echo ==^> Установка %APP_TITLE% в %INSTALL_DIR%...
mkdir "%INSTALL_DIR%" >nul 2>&1
copy /Y "%SCRIPT_DIR%sshfleet.exe" "%INSTALL_DIR%\sshfleet.exe" >nul

echo ==^> Создание ярлыков...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$ws = New-Object -ComObject WScript.Shell;" ^
    "$s = $ws.CreateShortcut([System.IO.Path]::Combine($env:USERPROFILE, 'Desktop', '%APP_TITLE%.lnk'));" ^
    "$s.TargetPath = '%INSTALL_DIR%\sshfleet.exe'; $s.IconLocation = '%INSTALL_DIR%\sshfleet.exe'; $s.Save();" ^
    "$sm = [System.IO.Path]::Combine($env:APPDATA, 'Microsoft\Windows\Start Menu\Programs', '%APP_TITLE%.lnk');" ^
    "$s2 = $ws.CreateShortcut($sm); $s2.TargetPath = '%INSTALL_DIR%\sshfleet.exe'; $s2.IconLocation = '%INSTALL_DIR%\sshfleet.exe'; $s2.Save();"

echo ==^> Готово. Запустите "%APP_TITLE%" с рабочего стола или из меню Пуск.
pause
