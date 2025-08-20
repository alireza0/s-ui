@echo off
setlocal enabledelayedexpansion

echo ========================================
echo S-UI Windows Installer
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

echo Installing S-UI to: %INSTALL_DIR%

REM Create installation directory
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%INSTALL_DIR%\db" mkdir "%INSTALL_DIR%\db"
if not exist "%INSTALL_DIR%\logs" mkdir "%INSTALL_DIR%\logs"
if not exist "%INSTALL_DIR%\cert" mkdir "%INSTALL_DIR%\cert"

REM Copy files
echo Copying files...
copy "sui.exe" "%INSTALL_DIR%\" >nul
copy "s-ui-windows.xml" "%INSTALL_DIR%\" >nul
copy "s-ui-windows.bat" "%INSTALL_DIR%\" >nul

REM Check if WinSW is available
set "WINSW_PATH=%INSTALL_DIR%\winsw.exe"
if not exist "%WINSW_PATH%" (
    echo Downloading WinSW...
    powershell -Command "& {Invoke-WebRequest -Uri 'https://github.com/winsw/winsw/releases/download/v2.12.0/WinSW-x64.exe' -OutFile '%WINSW_PATH%'}"
    if exist "%WINSW_PATH%" (
        echo WinSW downloaded successfully
    ) else (
        echo Warning: Failed to download WinSW. Service installation will be skipped.
        echo You can manually download WinSW from: https://github.com/winsw/winsw/releases
    )
)

REM Install Windows Service
if exist "%WINSW_PATH%" (
    echo Installing Windows Service...
    cd /d "%INSTALL_DIR%"
    copy "winsw.exe" "s-ui-service.exe" >nul
    copy "s-ui-windows.xml" "s-ui-service.xml" >nul
        
    REM Install service
    s-ui-service.exe install
    if %errorLevel% equ 0 (
        echo Service installed successfully
    ) else (
        echo Warning: Failed to install service. You can install it manually later.
    )
)

REM Run migration
echo Running database migration...
cd /d "%INSTALL_DIR%"
sui.exe migrate
if %errorLevel% equ 0 (
    echo Migration completed successfully
) else (
    echo Warning: Migration failed or database is new
)

REM Get network configuration
echo.
echo ========================================
echo Network Configuration
echo ========================================

REM Get local IP addresses
echo Available IP addresses:
for /f "tokens=2 delims=:" %%i in ('ipconfig ^| findstr /i "IPv4"') do (
    echo   %%i
)

REM Get panel configuration
echo.
set /p panel_port="Enter panel port (default: 2095): "
if "%panel_port%"=="" set "panel_port=2095"

set /p panel_path="Enter panel path (default: /app/): "
if "%panel_path%"=="" set "panel_path=/app/"

set /p sub_port="Enter subscription port (default: 2096): "
if "%sub_port%"=="" set "sub_port=2096"

set /p sub_path="Enter subscription path (default: /sub/): "
if "%sub_path%"=="" set "sub_path=/sub/"

REM Apply settings
echo.
echo Applying settings...
cd /d "%INSTALL_DIR%"
sui.exe setting -port %panel_port% -path "%panel_path%" -subPort %sub_port% -subPath "%sub_path%"

REM Get admin credentials
echo.
echo ========================================
echo Admin Configuration
echo ========================================

set /p admin_username="Enter admin username (default: admin): "
if "%admin_username%"=="" set "admin_username=admin"

set /p admin_password="Enter admin password: "
if "%admin_password%"=="" (
    echo Error: Password cannot be empty
    pause
    exit /b 1
)

REM Set admin credentials
echo Setting admin credentials...
sui.exe admin -username "%admin_username%" -password "%admin_password%"

REM Start service
echo Starting S-UI service...
net start %SERVICE_NAME%
if %errorLevel% equ 0 (
    echo Service started successfully
) else (
    echo Warning: Failed to start service. You can start it manually later.
)

REM Create desktop shortcut
echo Creating desktop shortcut...
set "DESKTOP=%USERPROFILE%\Desktop"
if exist "%DESKTOP%" (
    powershell -Command "& {$WshShell = New-Object -comObject WScript.Shell; $Shortcut = $WshShell.CreateShortcut('%DESKTOP%\S-UI.lnk'); $Shortcut.TargetPath = '%INSTALL_DIR%\s-ui-windows.bat'; $Shortcut.WorkingDirectory = '%INSTALL_DIR%'; $Shortcut.Description = 'S-UI Control Panel'; $Shortcut.Save()}"
    echo Desktop shortcut created
)

REM Create Start Menu shortcut
echo Creating Start Menu shortcut...
set "START_MENU=%APPDATA%\Microsoft\Windows\Start Menu\Programs"
if exist "%START_MENU%" (
    if not exist "%START_MENU%\S-UI" mkdir "%START_MENU%\S-UI"
    powershell -Command "& {$WshShell = New-Object -comObject WScript.Shell; $Shortcut = $WshShell.CreateShortcut('%START_MENU%\S-UI\S-UI Control Panel.lnk'); $Shortcut.TargetPath = '%INSTALL_DIR%\s-ui-windows.bat'; $Shortcut.WorkingDirectory = '%INSTALL_DIR%'; $Shortcut.Description = 'S-UI Control Panel'; $Shortcut.Save()}"
    echo Start Menu shortcut created
)

REM Set permissions
echo Setting permissions...
icacls "%INSTALL_DIR%" /grant "Users:(OI)(CI)RX" /T >nul
icacls "%INSTALL_DIR%\db" /grant "Users:(OI)(CI)F" /T >nul
icacls "%INSTALL_DIR%\logs" /grant "Users:(OI)(CI)F" /T >nul

REM Create environment variable
echo Setting environment variable...
setx SUI_HOME "%INSTALL_DIR%" /M >nul

REM Show final configuration
echo.
echo ========================================
echo Installation completed successfully!
echo ========================================
echo.
echo S-UI has been installed to: %INSTALL_DIR%
echo.
echo Configuration:
echo   Panel Port: %panel_port%
echo   Panel Path: %panel_path%
echo   Subscription Port: %sub_port%
echo   Subscription Path: %sub_path%
echo   Admin Username: %admin_username%
echo.
echo Access URLs:
for /f "tokens=2 delims=:" %%i in ('ipconfig ^| findstr /i "IPv4"') do (
    set "ip=%%i"
    set "ip=!ip: =!"
    echo   Panel: http://!ip!:%panel_port%%panel_path%
    echo   Subscription: http://!ip!:%sub_port%%sub_path%
)
echo.
echo Service name: %SERVICE_NAME%
echo.
echo Useful commands:
echo   net start %SERVICE_NAME%    - Start the service
echo   net stop %SERVICE_NAME%     - Stop the service
echo   sc query %SERVICE_NAME%     - Check service status
echo.
echo You can also use the desktop shortcut or Start Menu item.
echo.
pause
