Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-EnvOrDefault {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$Default
    )
    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) { return $Default }
    return $value.Trim()
}

function Invoke-Download {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][string]$OutFile
    )
    $params = @{
        Uri = $Uri
        OutFile = $OutFile
        ErrorAction = "Stop"
    }
    if ($PSVersionTable.PSVersion.Major -lt 6) {
        $params["UseBasicParsing"] = $true
    }
    Invoke-WebRequest @params
}

function Test-PathEntry {
    param(
        [AllowNull()][string]$PathValue,
        [Parameter(Mandatory = $true)][string]$Entry
    )
    if ([string]::IsNullOrWhiteSpace($PathValue)) { return $false }
    $target = $Entry.Trim().TrimEnd("\")
    foreach ($item in ($PathValue -split ";")) {
        if ($item.Trim().TrimEnd("\") -ieq $target) { return $true }
    }
    return $false
}

$repo = Get-EnvOrDefault "TASK_MECCA_REPO" "silverkhan/TaskMecca"
$tag = Get-EnvOrDefault "TASK_MECCA_RELEASE_TAG" "release-stable"
$asset = "task-mecca-windows-amd64.exe"
$baseUrl = "https://github.com/$repo/releases/download/$tag"

$arch = if (-not [string]::IsNullOrWhiteSpace($env:PROCESSOR_ARCHITEW6432)) {
    $env:PROCESSOR_ARCHITEW6432
} else {
    $env:PROCESSOR_ARCHITECTURE
}
if ($arch -notin @("AMD64", "x86_64")) {
    throw "Task Mecca install: unsupported Windows architecture: $arch (amd64 release required)."
}

$localAppData = $env:LOCALAPPDATA
if ([string]::IsNullOrWhiteSpace($localAppData)) {
    if ([string]::IsNullOrWhiteSpace($env:USERPROFILE)) {
        throw "Task Mecca install: LOCALAPPDATA and USERPROFILE are unavailable."
    }
    $localAppData = Join-Path $env:USERPROFILE "AppData\Local"
}

$installDir = Get-EnvOrDefault "TASK_MECCA_INSTALL_DIR" (Join-Path $localAppData "TaskMecca\bin")
$destination = Join-Path $installDir "task-mecca.exe"
$tempDir = Join-Path ([IO.Path]::GetTempPath()) ("task-mecca-install-" + [Guid]::NewGuid().ToString("N"))
$binaryPath = Join-Path $tempDir $asset
$checksumPath = Join-Path $tempDir "SHA256SUMS.txt"

New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
try {
    Write-Host "Downloading Task Mecca (Windows/amd64)..."
    Invoke-Download "$baseUrl/$asset" $binaryPath
    Invoke-Download "$baseUrl/SHA256SUMS.txt" $checksumPath

    $expected = $null
    foreach ($line in Get-Content -LiteralPath $checksumPath) {
        $parts = $line.Trim() -split "\s+", 2
        if ($parts.Count -ne 2) { continue }
        $name = $parts[1].Trim().TrimStart("*")
        if ([IO.Path]::GetFileName($name) -ieq $asset) {
            $expected = $parts[0].ToLowerInvariant()
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($expected)) {
        throw "Task Mecca install: checksum entry missing for $asset."
    }

    $actual = (Get-FileHash -LiteralPath $binaryPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "Task Mecca install: SHA-256 verification failed."
    }

    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Copy-Item -LiteralPath $binaryPath -Destination $destination -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not (Test-PathEntry $userPath $installDir)) {
        $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $installDir
        } else {
            "$installDir;$userPath"
        }
        try {
            [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
            Write-Host "Added to user PATH: $installDir"
        } catch {
            Write-Warning "Installed successfully, but the user PATH could not be updated automatically: $($_.Exception.Message)"
        }
    }

    if (-not (Test-PathEntry $env:Path $installDir)) {
        $env:Path = "$installDir;$env:Path"
    }

    Write-Host "Installed: $destination"
    & $destination --version

    Write-Host ""
    Write-Host "Project setup:"
    Write-Host "  task-mecca init"
} finally {
    Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
