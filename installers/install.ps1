$ErrorActionPreference = "Stop"

$BundledVersionFile = Join-Path $PSScriptRoot "VERSION"
$Version = if ($env:SPARE_VERSION) {
  $env:SPARE_VERSION
} elseif (Test-Path $BundledVersionFile) {
  (Get-Content $BundledVersionFile -Raw).Trim()
} else {
  "0.1.0"
}
$InstallDir = if ($env:SPARE_INSTALL_DIR) { $env:SPARE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Spare\bin" }
$BaseUrl = if ($env:SPARE_RELEASE_BASE_URL) { $env:SPARE_RELEASE_BASE_URL } else { "https://github.com/spare-run/spare/releases/download/v$Version" }

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$LocalSpare = Join-Path $ScriptDir "spare.exe"
$LocalDaemon = Join-Path $ScriptDir "spared.exe"
$RecipeDir = Join-Path $env:LOCALAPPDATA "Spare\recipes"

function Install-DefaultRecipes {
  param([string]$Source)

  New-Item -ItemType Directory -Force -Path $RecipeDir | Out-Null
  foreach ($RecipeId in @("site", "drop", "hook")) {
    $RecipePackage = Join-Path $Source ("{0}_{1}.sp" -f $RecipeId, $Version)
    if (-not (Test-Path $RecipePackage)) { throw "The release archive is missing $RecipePackage." }
    Copy-Item $RecipePackage (Join-Path $RecipeDir ("{0}_{1}.sp" -f $RecipeId, $Version)) -Force
  }
}

if ((Test-Path $LocalSpare) -and (Test-Path $LocalDaemon)) {
  Copy-Item $LocalSpare (Join-Path $InstallDir "spare.exe") -Force
  Copy-Item $LocalDaemon (Join-Path $InstallDir "spared.exe") -Force
  $RecipeSource = Join-Path $ScriptDir "recipes"
  Install-DefaultRecipes $RecipeSource
} else {
  $Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
  $Archive = "spare_${Version}_windows_${Arch}.zip"
  $TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("spare-" + [System.Guid]::NewGuid())
  New-Item -ItemType Directory -Path $TempDir | Out-Null
  try {
    Invoke-WebRequest "$BaseUrl/$Archive" -OutFile (Join-Path $TempDir $Archive)
    Invoke-WebRequest "$BaseUrl/checksums.txt" -OutFile (Join-Path $TempDir "checksums.txt")
    $ExpectedLine = Get-Content (Join-Path $TempDir "checksums.txt") | Where-Object { $_ -match [regex]::Escape($Archive) } | Select-Object -First 1
    if (-not $ExpectedLine) { throw "The release checksum is missing." }
    $Expected = ($ExpectedLine -split "\s+")[0].ToLowerInvariant()
    $Actual = (Get-FileHash (Join-Path $TempDir $Archive) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Expected -ne $Actual) { throw "The Spare archive checksum did not match." }
    Expand-Archive (Join-Path $TempDir $Archive) -DestinationPath $TempDir -Force
    Copy-Item (Join-Path $TempDir "spare.exe") (Join-Path $InstallDir "spare.exe") -Force
    Copy-Item (Join-Path $TempDir "spared.exe") (Join-Path $InstallDir "spared.exe") -Force
    $RecipeSource = Join-Path $TempDir "recipes"
    Install-DefaultRecipes $RecipeSource
  } finally {
    Remove-Item $TempDir -Recurse -Force -ErrorAction SilentlyContinue
  }
}

Write-Output "Installed the default recipe packages in $RecipeDir."

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($UserPath -split ";") -notcontains $InstallDir) {
  [Environment]::SetEnvironmentVariable("Path", (($UserPath.TrimEnd(";") + ";" + $InstallDir).TrimStart(";")), "User")
  $env:Path = "$env:Path;$InstallDir"
}

$SpareExe = Join-Path $InstallDir "spare.exe"
$ClassesRoot = "HKCU:\Software\Classes"
$ExtensionKey = Join-Path $ClassesRoot ".sp"
$RecipeClass = Join-Path $ClassesRoot "Spare.Recipe"
New-Item -Path $ExtensionKey -Force | Out-Null
$CurrentAssociation = (Get-Item $ExtensionKey).GetValue("")
if (-not $CurrentAssociation -or $CurrentAssociation -eq "Spare.Recipe") {
  Set-Item -Path $ExtensionKey -Value "Spare.Recipe"
}
New-ItemProperty -Path $ExtensionKey -Name "Content Type" -Value "application/vnd.spare.recipe+zip" -PropertyType String -Force | Out-Null
New-Item -Path $RecipeClass -Force | Out-Null
Set-Item -Path $RecipeClass -Value "Spare recipe package"
New-Item -Path (Join-Path $RecipeClass "DefaultIcon") -Force | Out-Null
Set-Item -Path (Join-Path $RecipeClass "DefaultIcon") -Value "`"$SpareExe`",0"
New-Item -Path (Join-Path $RecipeClass "shell\open\command") -Force | Out-Null
Set-Item -Path (Join-Path $RecipeClass "shell\open\command") -Value "`"$SpareExe`" view `"%1`""
Write-Output "Registered .sp files with Spare Recipe Viewer."

& $SpareExe init
