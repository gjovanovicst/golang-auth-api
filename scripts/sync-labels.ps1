#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Creates or updates the repository labels the project uses.

.DESCRIPTION
    Idempotent: existing labels are updated in place, missing ones are created.
    Requires the GitHub CLI (`gh`) authenticated with repository write access.

.PARAMETER Repo
    owner/repo. Defaults to the current repository (gh repo view).

.PARAMETER DryRun
    Print what would change without calling the API.

.EXAMPLE
    ./scripts/sync-labels.ps1 -DryRun

.EXAMPLE
    ./scripts/sync-labels.ps1 -Repo gjovanovicst/golang-auth-api
#>
[CmdletBinding()]
param(
    [string]$Repo = "",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    throw "GitHub CLI (gh) is required."
}

if ([string]::IsNullOrWhiteSpace($Repo)) {
    $Repo = (gh repo view --json nameWithOwner --jq '.nameWithOwner').Trim()
    if ([string]::IsNullOrWhiteSpace($Repo)) {
        throw "Could not determine the repository. Pass -Repo owner/repo."
    }
}

# name | color | description
$labels = @(
    @{ Name = "bug";              Color = "d73a4a"; Description = "Something is not working" },
    @{ Name = "feature";          Color = "a2eeef"; Description = "New capability or user-facing behaviour" },
    @{ Name = "improvement";      Color = "0e8a16"; Description = "Quality, performance or DX improvement" },
    @{ Name = "docs";             Color = "0075ca"; Description = "Documentation only" },
    @{ Name = "security";         Color = "b60205"; Description = "Security-related change or report (never details in public)" },
    @{ Name = "dependencies";     Color = "0366d6"; Description = "Dependency updates" },
    @{ Name = "ci";               Color = "ededed"; Description = "CI/CD and repository automation" },

    @{ Name = "area:api";         Color = "1d76db"; Description = "HTTP endpoints, middleware, DTOs" },
    @{ Name = "area:auth";        Color = "0e8a16"; Description = "Login, registration, tokens, password policy" },
    @{ Name = "area:2fa";         Color = "5319e7"; Description = "TOTP, email/SMS 2FA, passkeys, recovery" },
    @{ Name = "area:gui";         Color = "c5def5"; Description = "Admin GUI (HTMX templates)" },
    @{ Name = "area:email";       Color = "fbca04"; Description = "Email templates, SMTP, deliverability" },
    @{ Name = "area:oidc";        Color = "006b75"; Description = "OIDC provider, SSO and session groups" },
    @{ Name = "area:infra";       Color = "bfd4f2"; Description = "Docker, CI, migrations, configuration" },

    @{ Name = "p0";               Color = "b60205"; Description = "Drop everything" },
    @{ Name = "p1";               Color = "d93f0b"; Description = "Next" },
    @{ Name = "p2";               Color = "fbca04"; Description = "Later" },

    @{ Name = "triage";           Color = "d4c5f9"; Description = "Needs a decision or more information" },
    @{ Name = "ready";            Color = "0e8a16"; Description = "Groomed and ready to implement" },
    @{ Name = "blocked";          Color = "b60205"; Description = "Waiting on a decision or a dependency" },
    @{ Name = "breaking";         Color = "5319e7"; Description = "Breaking API or schema change" },
    @{ Name = "good first issue"; Color = "7057ff"; Description = "Good entry point for a new contributor" },
    @{ Name = "skip-changelog";   Color = "ededed"; Description = "The changelog check is intentionally skipped" },
    @{ Name = "backport/1.0";     Color = "1d76db"; Description = "Cherry-pick to release/1.0 on merge" }
)

Write-Host "Repository: $Repo"
if ($DryRun) { Write-Host "Mode:       dry run (no API calls)`n" -ForegroundColor Yellow }
else { Write-Host "Mode:       apply`n" }

$existing = @{}
$page = (gh api --paginate "repos/$Repo/labels?per_page=100" | ConvertFrom-Json)
foreach ($l in $page) { $existing[$l.name] = $l }

$created = 0
$updated = 0
$unchanged = 0

foreach ($l in $labels) {
    $name = $l.Name
    $color = $l.Color
    $desc = $l.Description
    $current = $existing[$name]

    if ($null -eq $current) {
        if ($DryRun) { Write-Host "  + create  $name" }
        else {
            gh api -X POST "repos/$Repo/labels" -f "name=$name" -f "color=$color" -f "description=$desc" | Out-Null
            Write-Host "  + created $name" -ForegroundColor Green
        }
        $created++
        continue
    }

    $colorDrift = $current.color -ne $color
    $descDrift = $current.description -ne $desc
    if (-not ($colorDrift -or $descDrift)) {
        $unchanged++
        continue
    }

    if ($DryRun) {
        Write-Host ("  ~ update  {0} (color {1}->{2}, description changed: {3})" -f $name, $current.color, $color, $descDrift)
    }
    else {
        $encoded = [uri]::EscapeDataString($name)
        gh api -X PATCH "repos/$Repo/labels/$encoded" -f "color=$color" -f "description=$desc" | Out-Null
        Write-Host "  ~ updated $name" -ForegroundColor Cyan
    }
    $updated++
}

Write-Host ""
Write-Host ("Summary: {0} created, {1} updated, {2} unchanged (of {3})." -f $created, $updated, $unchanged, $labels.Count)
if ($DryRun) { Write-Host "Run again without -DryRun to apply." }
