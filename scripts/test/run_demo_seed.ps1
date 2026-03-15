param(
    [Parameter(Mandatory = $true)]
    [string]$ShortCodes,

    [string]$Preset = "demo-medium",

    [string]$BaseUrl = "http://localhost:8080",

    [string]$WithHistory = "true",

    [string]$WithLiveToday = "true"
)

$ErrorActionPreference = "Stop"
$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")

function Get-PresetConfig {
    param([string]$Name)

    switch ($Name) {
        "demo-small" {
            return @{
                LiveIterations = 1200
                LiveVus = 6
                SleepSeconds = "0.06"
                IpCount = 240
                CnRatio = "0.62"
            }
        }
        "demo-medium" {
            return @{
                LiveIterations = 2800
                LiveVus = 8
                SleepSeconds = "0.04"
                IpCount = 420
                CnRatio = "0.62"
            }
        }
        "demo-large" {
            return @{
                LiveIterations = 5200
                LiveVus = 12
                SleepSeconds = "0.02"
                IpCount = 720
                CnRatio = "0.60"
            }
        }
        default {
            throw "Unsupported preset: $Name. Available values: demo-small, demo-medium, demo-large"
        }
    }
}

function To-Bool {
    param([string]$Value)

    switch ($Value.Trim().ToLower()) {
        "true" { return $true }
        "1" { return $true }
        "yes" { return $true }
        "on" { return $true }
        "false" { return $false }
        "0" { return $false }
        "no" { return $false }
        "off" { return $false }
        default { throw "Invalid boolean value: $Value. Use true/false." }
    }
}

function Split-ShortCodes {
    param([string]$Raw)

    return $Raw.Split(",") `
        | ForEach-Object { $_.Trim() } `
        | Where-Object { $_ -ne "" }
}

function Expand-WeightedValues {
    param([array]$Items)

    $expanded = New-Object System.Collections.Generic.List[string]
    foreach ($item in $Items) {
        for ($i = 0; $i -lt [int]$item.Weight; $i++) {
            $expanded.Add([string]$item.Value)
        }
    }
    return ($expanded -join ",")
}

function Get-WeightedShortCodeCsv {
    param([string[]]$Codes)

    $baseWeights = @(26, 21, 14, 10, 8, 6, 5, 4, 4, 3, 3, 3, 2, 2, 2, 2, 1, 1, 1)
    $items = @()
    for ($index = 0; $index -lt $Codes.Count; $index++) {
        $weight = 1
        if ($index -lt $baseWeights.Count) {
            $weight = $baseWeights[$index]
        }
        $items += @{
            Value = $Codes[$index]
            Weight = $weight
        }
    }
    return Expand-WeightedValues -Items $items
}

function Get-WeightedRefererCsv {
    $items = @(
        @{ Value = ""; Weight = 6 }
        @{ Value = "https://weixin.qq.com/s/demo"; Weight = 16 }
        @{ Value = "https://weibo.com/"; Weight = 14 }
        @{ Value = "https://www.zhihu.com/"; Weight = 13 }
        @{ Value = "https://www.xiaohongshu.com/"; Weight = 11 }
        @{ Value = "https://www.douyin.com/"; Weight = 11 }
        @{ Value = "https://www.bilibili.com/"; Weight = 9 }
        @{ Value = "https://www.qq.com/"; Weight = 9 }
        @{ Value = "https://www.xiaoheihe.cn/app/bbs/link"; Weight = 8 }
        @{ Value = "https://www.baidu.com/"; Weight = 8 }
        @{ Value = "https://tieba.baidu.com/"; Weight = 6 }
        @{ Value = "https://www.google.com/search?q=slink"; Weight = 9 }
        @{ Value = "https://www.bing.com/search?q=shortlink"; Weight = 5 }
        @{ Value = "https://www.reddit.com/"; Weight = 4 }
        @{ Value = "https://x.com/"; Weight = 4 }
        @{ Value = "https://www.facebook.com/"; Weight = 3 }
        @{ Value = "https://www.instagram.com/"; Weight = 3 }
        @{ Value = "https://www.youtube.com/"; Weight = 3 }
    )

    return Expand-WeightedValues -Items $items
}

$shortCodeList = Split-ShortCodes -Raw $ShortCodes
if ($shortCodeList.Count -eq 0) {
    throw 'ShortCodes cannot be empty. Example: -ShortCodes "52KD2a,nvGQWJQ0,lV86cWyd"'
}

$presetConfig = Get-PresetConfig -Name $Preset
$historyEnabled = To-Bool -Value $WithHistory
$liveEnabled = To-Bool -Value $WithLiveToday
$weightedShortCodes = Get-WeightedShortCodeCsv -Codes $shortCodeList
$weightedReferers = Get-WeightedRefererCsv

Push-Location $RepoRoot
try {
    if ($historyEnabled) {
        Write-Host "==> Seeding historical demo data..." -ForegroundColor Cyan
        & go run .\cmd\seed-demo-data --short-codes $ShortCodes --preset $Preset
    }

    if ($liveEnabled) {
        if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) {
            throw 'k6 was not found. Install k6 first or use -WithLiveToday false.'
        }

        Write-Host "==> Simulating live traffic for today..." -ForegroundColor Cyan
        & k6 run `
            --vus $presetConfig.LiveVus `
            --iterations $presetConfig.LiveIterations `
            -e "BASE_URL=$BaseUrl" `
            -e "SHORT_CODES=$weightedShortCodes" `
            -e "REFERER_POOL=$weightedReferers" `
            -e "IP_COUNT=$($presetConfig.IpCount)" `
            -e "CN_RATIO=$($presetConfig.CnRatio)" `
            -e "SLEEP_SECONDS=$($presetConfig.SleepSeconds)" `
            .\scripts\test\seed_access_data.js
    }

    Write-Host "==> Demo seeding completed." -ForegroundColor Green
}
finally {
    Pop-Location
}
