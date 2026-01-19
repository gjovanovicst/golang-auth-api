@echo off
setlocal

:: Get the directory where the script is located
set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR%.."
set "BACKUP_DIR=%PROJECT_ROOT%\backups"

:: Create backups directory if it doesn't exist
if not exist "%BACKUP_DIR%" mkdir "%BACKUP_DIR%"

:: Load .env file
if exist "%PROJECT_ROOT%\.env" (
    for /f "usebackq tokens=1* delims==" %%A in ("%PROJECT_ROOT%\.env") do (
        if not "%%A"=="" if not "%%B"=="" set "%%A=%%B"
    )
)

:: Default values if not set in .env
if "%DB_USER%"=="" set DB_USER=postgres
if "%DB_PASSWORD%"=="" set DB_PASSWORD=root
if "%DB_NAME%"=="" set DB_NAME=auth_db
set CONTAINER_NAME=auth_db

:: Timestamp for the backup file
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /value') do set datetime=%%I
set "TIMESTAMP=%datetime:~0,8%_%datetime:~8,6%"
set "BACKUP_FILE=%BACKUP_DIR%\%DB_NAME%_backup_%TIMESTAMP%.sql"

echo Creating backup of %DB_NAME%...
echo Target file: %BACKUP_FILE%

:: Check if container is running
docker ps | findstr "%CONTAINER_NAME%" >nul
if %errorlevel% neq 0 (
    echo Error: %CONTAINER_NAME% container is not running.
    exit /b 1
)

:: Execute pg_dump inside the container
docker exec -e PGPASSWORD=%DB_PASSWORD% -t %CONTAINER_NAME% pg_dump -U %DB_USER% %DB_NAME% > "%BACKUP_FILE%"

if %errorlevel% equ 0 (
    echo Backup created successfully!
    for %%I in ("%BACKUP_FILE%") do echo Size: %%~zI bytes
) else (
    echo Backup failed!
    if exist "%BACKUP_FILE%" del "%BACKUP_FILE%"
    exit /b 1
)

endlocal
