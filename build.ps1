[CmdletBinding()]
param(
    [ValidateSet("local", "arm64", "all")]
    [string]$Target = "local",
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

if (-not $Version) {
    try {
        $gitVer = (& "C:\Program Files\Git\cmd\git.exe" describe --tags --always --dirty 2>$null)
        if ($gitVer) { $Version = $gitVer.Trim() }
    } catch {
        $Version = "v0.1.0-dev"
    }
}
if (-not $Version) { $Version = "v0.1.0-dev" }

try {
    $gitCommit = (& "C:\Program Files\Git\cmd\git.exe" rev-parse --short HEAD 2>$null)
    if ($gitCommit) { $Commit = $gitCommit.Trim() } else { $Commit = "none" }
} catch {
    $Commit = "none"
}

$Date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$Pkg = "github.com/CarlosArguelloDev/okshield/internal/version"
$LdFlags = "-X $Pkg.Version=$Version -X $Pkg.Commit=$Commit -X $Pkg.Date=$Date -s -w"

Write-Host "Building OkShield version=$Version commit=$Commit date=$Date" -ForegroundColor Cyan

if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

$env:CGO_ENABLED = "0"

if ($Target -eq "local" -or $Target -eq "all") {
    Write-Host "Compiling local binary..." -ForegroundColor Green
    go build -ldflags $LdFlags -o bin/okshield.exe ./cmd/okshield
    Write-Host "Built bin/okshield.exe" -ForegroundColor Green
}

if ($Target -eq "arm64" -or $Target -eq "all") {
    Write-Host "Compiling Linux ARM64 binary..." -ForegroundColor Green
    $env:GOOS = "linux"
    $env:GOARCH = "arm64"
    go build -ldflags $LdFlags -o bin/okshield-linux-arm64 ./cmd/okshield
    $env:GOOS = ""
    $env:GOARCH = ""
    Write-Host "Built bin/okshield-linux-arm64" -ForegroundColor Green
}
