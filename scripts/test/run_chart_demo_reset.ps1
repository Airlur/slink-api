param(
    [string]$ShortCodes = "52KD2a,nvGQWJQ0,lV86cWyd,Doubao,13dsw4,89sdss,7d2aPPN4,i652zhFD6xI,leNMP4LV,gIYc1lNwbGO",

    [string]$Targets = "52KD2a=820,nvGQWJQ0=620,lV86cWyd=430,Doubao=320,13dsw4=240,89sdss=160,7d2aPPN4=110,i652zhFD6xI=80,leNMP4LV=70,gIYc1lNwbGO=50",

    [switch]$CleanupOnly,

    [switch]$SeedOnly,

    [switch]$RebuildOnly
)

$ErrorActionPreference = "Stop"
$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")

Push-Location $RepoRoot
try {
    $args = @(
        "run",
        ".\cmd\curate-demo-data",
        "--short-codes", $ShortCodes,
        "--targets", $Targets
    )

    if ($CleanupOnly) {
        $args += "--cleanup-only"
    }

    if ($SeedOnly) {
        $args += "--seed-only"
    }

    if ($RebuildOnly) {
        $args += "--rebuild-only"
    }

    & go @args
}
finally {
    Pop-Location
}
