@echo off
setlocal enabledelayedexpansion

echo ========================================
echo S-UI Windows Uninstaller
echo ========================================

REM Check if running as Administrator
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo Error: This script must be run as Administrator
    echo Right-click on this file and select "Run as administrator"
    pause
    exit /b 1
)

REM Set installation directory
set "INSTALL_DIR=C:\Program Files\s-ui"
set "SERVICE_NAME=s-ui"

echo Uninstalling S-UI from: %INSTALL_DIR%

REM Stop and remove Windows Service
if exist "%INSTALL_DIR%\s-ui-service.exe" (
    echo Stopping and removing Windows Service...
    net stop %SERVICE_NAME% >nul 2>&1
    cd /d "%INSTALL_DIR%"
    s-ui-service.exe uninstall >nul 2>&1
    if %errorLevel% equ 0 (
        echo Service removed successfully
    ) else (
        echo Warning: Failed to remove service or service was not installed
    )
)

REM Remove desktop shortcut
echo Removing desktop shortcut...
set "DESKTOP=%USERPROFILE%\Desktop"
if exist "%DESKTOP%\S-UI.lnk" (
    del "%DESKTOP%\S-UI.lnk" >nul 2>&1
    echo Desktop shortcut removed
)

REM Remove Start Menu shortcut
echo Removing Start Menu shortcut...
set "START_MENU=%APPDATA%\Microsoft\Windows\Start Menu\Programs\S-UI"
if exist "%START_MENU%" (
    rmdir /s /q "%START_MENU%" >nul 2>&1
    echo Start Menu shortcut removed
)

REM Remove environment variable
echo Removing environment variable...
reg delete "HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Session Manager\Environment" /v SUI_HOME /f >nul 2>&1

REM Ask user if they want to keep data
echo.
set /p keep_data="Do you want to keep your data (database, logs, certificates)? [y/n]: "
if /i "%keep_data%"=="y" (
    echo Keeping data files...
    REM Remove only executable and service files
    if exist "%INSTALL_DIR%\sui.exe" del "%INSTALL_DIR%\sui.exe" >nul 2>&1
    if exist "%INSTALL_DIR%\s-ui-service.exe" del "%INSTALL_DIR%\s-ui-service.exe" >nul 2>&1
    if exist "%INSTALL_DIR%\s-ui-service.xml" del "%INSTALL_DIR%\s-ui-service.xml" >nul 2>&1
    if exist "%INSTALL_DIR%\winsw.exe" del "%INSTALL_DIR%\winsw.exe" >nul 2>&1
    if exist "%INSTALL_DIR%\*.bat" del "%INSTALL_DIR%\*.bat" >nul 2>&1
    if exist "%INSTALL_DIR%\*.xml" del "%INSTALL_DIR%\*.xml" >nul 2>&1
    if exist "%INSTALL_DIR%\*.md" del "%INSTALL_DIR%\*.md" >nul 2>&1
    echo Data files preserved in: %INSTALL_DIR%
) else (
    echo Removing all files...
    REM Remove entire installation directory
    if exist "%INSTALL_DIR%" (
        rmdir /s /q "%INSTALL_DIR%" >nul 2>&1
        if exist "%INSTALL_DIR%" (
            echo Warning: Some files could not be removed. Please manually delete: %INSTALL_DIR%
        ) else (
            echo All files removed successfully
        )
    )
)

REM Remove firewall rules
echo Removing firewall rules...
netsh advfirewall firewall delete rule name="S-UI Panel" >nul 2>&1
netsh advfirewall firewall delete rule name="S-UI Subscription" >nul 2>&1

echo.
echo ========================================
echo Uninstallation completed!
echo ========================================
echo.
echo S-UI has been uninstalled from your system.
echo.
if /i "%keep_data%"=="y" (
    echo Your data has been preserved in: %INSTALL_DIR%
    echo You can safely delete this directory if you no longer need the data.
)
echo.
echo Thank you for using S-UI!
echo.
pause
