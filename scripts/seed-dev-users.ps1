#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Seeds deterministic local development users into the Auth API database.

.DESCRIPTION
    Creates (or repairs) two application-level test users for local development and
    manual regression testing:

        dev@example.com       verified email, password only (no 2FA)
        dev-2fa@example.com   verified email + TOTP 2FA + verified phone number

    Registration goes through the public API (POST /register) so password hashing
    and validation stay inside the application. Email/phone verification and the
    TOTP secret are applied directly in PostgreSQL because no SMTP or SMS provider
    is configured locally.

    The script is idempotent: re-running it re-applies the flags and leaves existing
    password hashes untouched.

.PARAMETER BaseUrl
    Base URL of the running API. Default: http://localhost:8080

.PARAMETER AppId
    Application the users belong to. Default: 00000000-0000-0000-0000-000000000001

.PARAMETER PsqlPath
    Path to psql.exe. Default: C:\Program Files\PostgreSQL\17\bin\psql.exe

.EXAMPLE
    ./scripts/seed-dev-users.ps1

.EXAMPLE
    ./scripts/seed-dev-users.ps1 -BaseUrl http://localhost:8090 -DbPort 5433
#>
[CmdletBinding()]
param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$AppId = "00000000-0000-0000-0000-000000000001",

    [string]$DbHost = "localhost",
    [int]$DbPort = 5432,
    [string]$DbName = "auth_db",
    [string]$DbUser = "postgres",
    [string]$DbPassword = "root",
    [string]$PsqlPath = "C:\Program Files\PostgreSQL\17\bin\psql.exe",

    [string]$DevEmail = "dev@example.com",
    [string]$DevPassword = "Dev!Pass-2026-alpha",
    [string]$TwoFAEmail = "dev-2fa@example.com",
    [string]$TwoFAPassword = "Dev!Pass-2026-bravo",
    [string]$TwoFAPhone = "+381600000001",
    [string]$TotpSecret = "JBSWY3DPEHPK3PXP"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $PsqlPath)) {
    throw "psql not found at '$PsqlPath'. Pass -PsqlPath explicitly."
}

$Headers = @{
    "X-App-ID"     = $AppId
    "Content-Type" = "application/json"
}

function Invoke-Sql {
    param([Parameter(Mandatory)][string]$Sql)

    $previous = $env:PGPASSWORD
    $env:PGPASSWORD = $DbPassword
    try {
        $output = & $PsqlPath -h $DbHost -p $DbPort -U $DbUser -d $DbName -v ON_ERROR_STOP=1 -tAc $Sql 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "psql failed: $($output -join ' ')"
        }
        return (($output | Out-String).Trim())
    }
    finally {
        $env:PGPASSWORD = $previous
    }
}

function Register-AppUser {
    param(
        [Parameter(Mandatory)][string]$Email,
        [Parameter(Mandatory)][string]$Password
    )

    # Skip the API call when the account already exists: registration is rate
    # limited per IP, and re-creating users is pointless for a seeder.
    $existing = Invoke-Sql "SELECT count(*) FROM users WHERE email = '$Email' AND app_id = '$AppId'"
    if ($existing -eq "1") {
        Write-Host "  [exists]   $Email"
        return
    }

    $payload = @{ email = $Email; password = $Password } | ConvertTo-Json -Compress
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        try {
            Invoke-RestMethod -Method Post -Uri "$BaseUrl/register" -Headers $Headers `
                -ContentType "application/json" -Body $payload -TimeoutSec 30 | Out-Null
            Write-Host "  [created]  $Email"
            return
        }
        catch {
            $detail = $_.ErrorDetails.Message
            if ($detail -match "Too many requests" -and $attempt -lt 3) {
                Write-Host "  [wait]     rate limited, retrying in 10s..."
                Start-Sleep -Seconds 10
                continue
            }
            if ($detail -match "already|exists|registered|duplicate") {
                Write-Host "  [exists]   $Email"
                return
            }
            throw "Registration of $Email failed: $detail"
        }
    }
}

Write-Host "Auth API dev user seeder"
Write-Host "  API : $BaseUrl (app $AppId)"
Write-Host "  DB  : ${DbHost}:${DbPort}/${DbName}"
Write-Host ""

# --- 1. Plain user: verified email, no 2FA ---------------------------------
Write-Host "dev user:"
Register-AppUser -Email $DevEmail -Password $DevPassword
Invoke-Sql @"
UPDATE users
   SET email_verified = true,
       two_fa_enabled = false,
       two_fa_method = '',
       two_fa_secret = '',
       two_fa_recovery_codes = NULL
 WHERE email = '$DevEmail'
"@ | Out-Null

# --- 2. 2FA user: verified email + TOTP + verified phone -------------------
Write-Host "2FA user:"
Register-AppUser -Email $TwoFAEmail -Password $TwoFAPassword
$recoveryCodesJson = '["0a1b2c3d4e5f6071","1122334455667788","99aabbccddeeff00"]'
Invoke-Sql @"
UPDATE users
   SET email_verified = true,
       phone_number = '$TwoFAPhone',
       phone_verified = true,
       two_fa_enabled = true,
       two_fa_method = 'totp',
       two_fa_secret = '$TotpSecret',
       two_fa_recovery_codes = '$recoveryCodesJson'::jsonb
 WHERE email = '$TwoFAEmail'
"@ | Out-Null

# --- 3. Verification (skipped gracefully when the API is not running) ------
Write-Host ""
Write-Host "Verification:"
try {
    $devBody = @{ email = $DevEmail; password = $DevPassword } | ConvertTo-Json -Compress
    $devLogin = Invoke-RestMethod -Method Post -Uri "$BaseUrl/login" -Headers $Headers `
        -ContentType "application/json" -Body $devBody -TimeoutSec 30
    if ($devLogin.access_token) { Write-Host "  [ok] $DevEmail can log in (JWT issued)" }

    $twoFaBody = @{ email = $TwoFAEmail; password = $TwoFAPassword } | ConvertTo-Json -Compress
    $twoFaLogin = Invoke-RestMethod -Method Post -Uri "$BaseUrl/login" -Headers $Headers `
        -ContentType "application/json" -Body $twoFaBody -TimeoutSec 30
    if ($twoFaLogin.requires_2fa) {
        Write-Host "  [ok] $TwoFAEmail requires 2FA (method: $($twoFaLogin.method))"
    }
}
catch {
    Write-Warning "Login verification skipped: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "Seeded accounts"
Write-Host "  ---------------------------------------------------------------"
Write-Host ("  {0,-22} {1}" -f $DevEmail, $DevPassword)
Write-Host ("  {0,-22} {1}   (TOTP + phone $TwoFAPhone)" -f $TwoFAEmail, $TwoFAPassword)
Write-Host "  ---------------------------------------------------------------"
Write-Host "  TOTP secret : $TotpSecret"
Write-Host "  otpauth URI : otpauth://totp/Auth%20API:${TwoFAEmail}?secret=$TotpSecret&issuer=Auth%20API"
Write-Host "  Recovery    : 0a1b2c3d4e5f6071, 1122334455667788, 99aabbccddeeff00"
Write-Host ""
Write-Host "Passwords are dev-only; change or delete these accounts before deploying."
