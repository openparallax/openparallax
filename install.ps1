# OpenParallax installer for Windows.
# Usage: irm https://get.openparallax.dev/install.ps1 | iex
#    or: .\install.ps1 [-Version v0.1.0] [-Dir "$env:LOCALAPPDATA\openparallax"]
param(
    [string]$Version = "",
    [string]$Dir = "$env:LOCALAPPDATA\openparallax\bin"
)

$ErrorActionPreference = "Stop"
$Repo = "openparallax/openparallax"

# Detect architecture.
$Arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Error "32-bit systems are not supported."
    exit 1
}

# Fetch latest version if not pinned.
if (-not $Version) {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $Release.tag_name
    if (-not $Version) {
        Write-Error "Failed to fetch latest release version."
        exit 1
    }
}

$VersionNum = $Version.TrimStart("v")
$Archive = "openparallax_${VersionNum}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Archive"
$ChecksumUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"

Write-Host "Installing OpenParallax $Version (windows/$Arch)"
Write-Host "  Archive:  $Archive"
Write-Host "  Install:  $Dir"

# Create temp directory.
$Tmp = New-TemporaryFile | ForEach-Object {
    Remove-Item $_
    New-Item -ItemType Directory -Path "$_.dir"
}

try {
    # Download.
    Write-Host "Downloading..."
    Invoke-WebRequest -Uri $Url -OutFile "$Tmp\$Archive"
    Invoke-WebRequest -Uri $ChecksumUrl -OutFile "$Tmp\checksums.txt"

    # Verify checksum.
    Write-Host "Verifying checksum..."
    $Expected = (Get-Content "$Tmp\checksums.txt" | Where-Object { $_ -match $Archive }) -split '\s+' | Select-Object -First 1
    if (-not $Expected) {
        Write-Error "Checksum not found for $Archive"
        exit 1
    }
    $Actual = (Get-FileHash -Algorithm SHA256 "$Tmp\$Archive").Hash.ToLower()
    if ($Expected -ne $Actual) {
        Write-Error "Checksum mismatch! Expected: $Expected Got: $Actual"
        exit 1
    }
    Write-Host "Checksum OK."

    # Extract.
    Expand-Archive -Path "$Tmp\$Archive" -DestinationPath $Tmp -Force

    # Install.
    New-Item -ItemType Directory -Path $Dir -Force | Out-Null
    Copy-Item "$Tmp\openparallax.exe" "$Dir\openparallax.exe" -Force

    Write-Host ""
    Write-Host "OpenParallax $Version installed to $Dir\openparallax.exe"

    # Check PATH.
    $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($UserPath -notlike "*$Dir*") {
        Write-Host ""
        Write-Host "Add to PATH (run once):"
        Write-Host "  [Environment]::SetEnvironmentVariable('PATH', `"$Dir;`$env:PATH`", 'User')"
    }

    Write-Host ""
    Write-Host "Get started:"
    Write-Host "  openparallax init"
    Write-Host "  openparallax start"
}
finally {
    Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
}
