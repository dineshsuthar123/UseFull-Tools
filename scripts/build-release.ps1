[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$Version = '0.2.0'
$RepositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$DistPath = [System.IO.Path]::GetFullPath((Join-Path $RepositoryRoot 'dist'))
$ExpectedDistParent = $RepositoryRoot
$ActualDistParent = [System.IO.Path]::GetDirectoryName($DistPath)

if (-not [string]::Equals($ActualDistParent, $ExpectedDistParent, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to clean unexpected distribution path: $DistPath"
}

$VersionSource = Get-Content -LiteralPath (Join-Path $RepositoryRoot 'internal/cli/cli.go') -Raw
$ExpectedVersionDeclaration = 'const Version = "' + $Version + '"'
if (-not $VersionSource.Contains($ExpectedVersionDeclaration)) {
    throw "CLI version does not match release version $Version"
}

$Targets = @(
    [pscustomobject]@{ GOOS = 'windows'; GOARCH = 'amd64'; Name = 'what-changed-windows-amd64.exe' }
    [pscustomobject]@{ GOOS = 'linux';   GOARCH = 'amd64'; Name = 'what-changed-linux-amd64' }
    [pscustomobject]@{ GOOS = 'linux';   GOARCH = 'arm64'; Name = 'what-changed-linux-arm64' }
    [pscustomobject]@{ GOOS = 'darwin';  GOARCH = 'amd64'; Name = 'what-changed-darwin-amd64' }
    [pscustomobject]@{ GOOS = 'darwin';  GOARCH = 'arm64'; Name = 'what-changed-darwin-arm64' }
)

if (Test-Path -LiteralPath $DistPath) {
    Remove-Item -LiteralPath $DistPath -Recurse -Force
}
New-Item -ItemType Directory -Path $DistPath | Out-Null

$PreviousCGO = [System.Environment]::GetEnvironmentVariable('CGO_ENABLED', 'Process')
$PreviousGOOS = [System.Environment]::GetEnvironmentVariable('GOOS', 'Process')
$PreviousGOARCH = [System.Environment]::GetEnvironmentVariable('GOARCH', 'Process')

Push-Location $RepositoryRoot
try {
    $env:CGO_ENABLED = '0'

    foreach ($Target in $Targets) {
        $env:GOOS = $Target.GOOS
        $env:GOARCH = $Target.GOARCH
        $OutputPath = Join-Path $DistPath $Target.Name

        Write-Host "Building $($Target.Name) for $($Target.GOOS)/$($Target.GOARCH)..."
        & go build -trimpath -buildvcs=false -ldflags '-s -w' -o $OutputPath ./cmd/what-changed
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed for $($Target.GOOS)/$($Target.GOARCH)"
        }

        $Artifact = Get-Item -LiteralPath $OutputPath
        if ($Artifact.Length -eq 0) {
            throw "Build produced an empty artifact: $($Target.Name)"
        }
    }
}
finally {
    Pop-Location
    [System.Environment]::SetEnvironmentVariable('CGO_ENABLED', $PreviousCGO, 'Process')
    [System.Environment]::SetEnvironmentVariable('GOOS', $PreviousGOOS, 'Process')
    [System.Environment]::SetEnvironmentVariable('GOARCH', $PreviousGOARCH, 'Process')
}

$ChecksumLines = Get-ChildItem -LiteralPath $DistPath -File |
    Sort-Object -Property Name |
    ForEach-Object {
        $Hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        "$Hash  $($_.Name)"
    }

$ChecksumPath = Join-Path $DistPath 'checksums.txt'
$Utf8NoBom = [System.Text.UTF8Encoding]::new($false)
[System.IO.File]::WriteAllLines($ChecksumPath, $ChecksumLines, $Utf8NoBom)

Write-Host "Built WhatChanged v$Version release assets in $DistPath"
