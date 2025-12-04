@echo off
REM Database Migration Script for Windows
REM Interactive tool for managing database migrations

setlocal EnableDelayedExpansion

REM Database connection (can be overridden by environment)
if not defined DB_HOST set DB_HOST=localhost
if not defined DB_PORT set DB_PORT=5433
if not defined DB_USER set DB_USER=postgres
if not defined DB_NAME set DB_NAME=auth_db

echo ======================================
echo Database Migration Tool
echo ======================================
echo.

REM Check if psql is available
where psql >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Error: psql command not found. Please install PostgreSQL client.
    exit /b 1
)

echo Current Configuration:
echo Host: %DB_HOST%
echo Port: %DB_PORT%
echo User: %DB_USER%
echo Database: %DB_NAME%
echo.

echo ======================================
echo Migration Options:
echo ======================================
echo.
echo 1. Show migration status
echo 2. Apply smart logging migration (v1.1.0)
echo 3. Rollback smart logging migration
echo 4. List all available migrations
echo 5. Backup database
echo 6. Test database connection
echo 0. Exit
echo.

set /p choice="Enter your choice (0-6): "

if "%choice%"=="1" goto show_status
if "%choice%"=="2" goto apply_migration
if "%choice%"=="3" goto rollback_migration
if "%choice%"=="4" goto list_migrations
if "%choice%"=="5" goto backup_database
if "%choice%"=="6" goto test_connection
if "%choice%"=="0" goto exit_script
goto invalid_choice

:show_status
echo.
echo Current Database Status:
echo.

REM Check if activity_logs table exists
psql -h %DB_HOST% -p %DB_PORT% -U %DB_USER% -d %DB_NAME% -c "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'activity_logs') as activity_logs_exists;"

REM Check smart logging fields
psql -h %DB_HOST% -p %DB_PORT% -U %DB_USER% -d %DB_NAME% -c "SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'activity_logs' AND column_name IN ('severity', 'expires_at', 'is_anomaly') ORDER BY column_name;"

REM Count logs
psql -h %DB_HOST% -p %DB_PORT% -U %DB_USER% -d %DB_NAME% -c "SELECT COUNT(*) as total_logs, pg_size_pretty(pg_total_relation_size('activity_logs')) as table_size FROM activity_logs;"

goto end

:apply_migration
echo.
echo Applying Smart Logging Migration (v1.1.0)
echo.

if not exist "migrations\20240103_add_activity_log_smart_fields.sql" (
    echo Error: Migration file not found!
    echo Expected: migrations\20240103_add_activity_log_smart_fields.sql
    exit /b 1
)

echo This will:
echo - Add severity field to activity_logs
echo - Add expires_at field to activity_logs
echo - Add is_anomaly field to activity_logs
echo - Create new indexes
echo - Update existing logs with defaults
echo.

set /p confirm="Apply migration? (yes/no): "
if not "%confirm%"=="yes" (
    echo Operation cancelled.
    goto end
)

echo.
echo Creating backup first...
for /f "tokens=2-4 delims=/ " %%a in ('date /t') do (set mydate=%%c%%a%%b)
for /f "tokens=1-2 delims=/:" %%a in ('time /t') do (set mytime=%%a%%b)
set timestamp=%mydate%_%mytime%
set backup_file=backup_before_smart_logging_%timestamp%.sql

pg_dump -h %DB_HOST% -p %DB_PORT% -U %DB_USER% %DB_NAME% > %backup_file%
echo Backup saved to: %backup_file%
echo.

echo Applying migration...
psql -h %DB_HOST% -p %DB_PORT% -U %DB_USER% -d %DB_NAME% -f migrations\20240103_add_activity_log_smart_fields.sql

if %ERRORLEVEL% EQU 0 (
    echo.
    echo Migration completed successfully!
) else (
    echo.
    echo Migration failed! Check errors above.
    exit /b 1
)

goto end

:rollback_migration
echo.
echo Rollback Smart Logging Migration
echo.

if not exist "migrations\20240103_add_activity_log_smart_fields_rollback.sql" (
    echo Error: Rollback file not found!
    exit /b 1
)

echo WARNING: This will remove smart logging fields!
echo.

set /p confirm="Are you sure you want to rollback? (yes/no): "
if not "%confirm%"=="yes" (
    echo Operation cancelled.
    goto end
)

echo.
echo Creating backup first...
for /f "tokens=2-4 delims=/ " %%a in ('date /t') do (set mydate=%%c%%a%%b)
for /f "tokens=1-2 delims=/:" %%a in ('time /t') do (set mytime=%%a%%b)
set timestamp=%mydate%_%mytime%
set backup_file=backup_before_rollback_%timestamp%.sql

pg_dump -h %DB_HOST% -p %DB_PORT% -U %DB_USER% %DB_NAME% > %backup_file%
echo Backup saved to: %backup_file%
echo.

echo Rolling back...
psql -h %DB_HOST% -p %DB_PORT% -U %DB_USER% -d %DB_NAME% -f migrations\20240103_add_activity_log_smart_fields_rollback.sql

if %ERRORLEVEL% EQU 0 (
    echo.
    echo Rollback completed!
) else (
    echo.
    echo Rollback failed! Check errors above.
    exit /b 1
)

goto end

:list_migrations
echo.
echo Available Migrations:
echo.

if exist "migrations" (
    echo Migrations in migrations\ directory:
    echo.
    dir /b migrations\*.sql 2>nul
    if %ERRORLEVEL% NEQ 0 (
        echo No migration files found
    )
) else (
    echo Migrations directory not found!
)

goto end

:backup_database
echo.
echo Creating Database Backup
echo.

for /f "tokens=2-4 delims=/ " %%a in ('date /t') do (set mydate=%%c%%a%%b)
for /f "tokens=1-2 delims=/:" %%a in ('time /t') do (set mytime=%%a%%b)
set timestamp=%mydate%_%mytime%
set backup_file=backup_%DB_NAME%_%timestamp%.sql

echo Backing up to: %backup_file%
echo.

pg_dump -h %DB_HOST% -p %DB_PORT% -U %DB_USER% %DB_NAME% > %backup_file%

if %ERRORLEVEL% EQU 0 (
    echo Backup created successfully!
    echo File: %backup_file%
) else (
    echo Backup failed!
    exit /b 1
)

goto end

:test_connection
echo.
echo Testing Database Connection
echo.

echo Host: %DB_HOST%
echo Port: %DB_PORT%
echo User: %DB_USER%
echo Database: %DB_NAME%
echo.

psql -h %DB_HOST% -p %DB_PORT% -U %DB_USER% -d %DB_NAME% -c "SELECT version();" >nul 2>nul

if %ERRORLEVEL% EQU 0 (
    echo [OK] Connection successful!
    echo.
    psql -h %DB_HOST% -p %DB_PORT% -U %DB_USER% -d %DB_NAME% -c "SELECT version();"
) else (
    echo [FAIL] Connection failed!
    echo.
    echo Please check:
    echo - Database is running
    echo - Host and port are correct
    echo - Username and password are correct
    echo - Database exists
    exit /b 1
)

goto end

:invalid_choice
echo Invalid choice. Exiting.
exit /b 1

:exit_script
echo Exiting...
exit /b 0

:end
echo.
echo ======================================
echo Operation completed!
echo ======================================
pause

