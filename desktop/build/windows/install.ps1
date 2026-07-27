param(
  [string]$InstallDir = $(if ($env:SPARE_DESKTOP_INSTALL_DIR) {
    $env:SPARE_DESKTOP_INSTALL_DIR
  } else {
    Join-Path $env:LOCALAPPDATA "Programs\Spare"
  })
)

$ErrorActionPreference = "Stop"
$InstallDir = [System.IO.Path]::GetFullPath($InstallDir)
$InstallRoot = [System.IO.Path]::GetPathRoot($InstallDir)
if ($InstallDir -eq $InstallRoot -or $InstallDir.TrimEnd("\") -eq $env:USERPROFILE.TrimEnd("\")) {
  throw "Refusing unsafe Spare install directory: $InstallDir"
}

$Required = @("Spare.exe", "spared.exe", "uninstall.ps1")
foreach ($Name in $Required) {
  if (-not (Test-Path (Join-Path $PSScriptRoot $Name))) {
    throw "The Spare Desktop package is missing $Name."
  }
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $InstallDir "bin") | Out-Null
foreach ($Name in $Required) {
  Copy-Item (Join-Path $PSScriptRoot $Name) (Join-Path $InstallDir $Name) -Force
}
if (-not (Test-Path (Join-Path $PSScriptRoot "bin\spare.exe"))) {
  throw "The Spare Desktop package is missing bin\spare.exe."
}
Copy-Item (Join-Path $PSScriptRoot "bin\spare.exe") (Join-Path $InstallDir "bin\spare.exe") -Force
Copy-Item (Join-Path $PSScriptRoot "VERSION") (Join-Path $InstallDir "VERSION") -Force
Copy-Item (Join-Path $PSScriptRoot "recipes") (Join-Path $InstallDir "recipes") -Recurse -Force

$BinDir = Join-Path $InstallDir "bin"
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$PathEntries = @($UserPath -split ";" | Where-Object { $_ })
if ($PathEntries -notcontains $BinDir) {
  $PathEntries += $BinDir
  [Environment]::SetEnvironmentVariable("Path", ($PathEntries -join ";"), "User")
}

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
New-Item -Path (Join-Path $RecipeClass "shell\open\command") -Force | Out-Null
Set-Item -Path (Join-Path $RecipeClass "shell\open\command") -Value "`"$(Join-Path $InstallDir 'Spare.exe')`" `"%1`""

$BackupExtensionKey = Join-Path $ClassesRoot ".spare-backup"
$BackupClass = Join-Path $ClassesRoot "Spare.Backup"
New-Item -Path $BackupExtensionKey -Force | Out-Null
$CurrentBackupAssociation = (Get-Item $BackupExtensionKey).GetValue("")
if (-not $CurrentBackupAssociation -or $CurrentBackupAssociation -eq "Spare.Backup") {
  Set-Item -Path $BackupExtensionKey -Value "Spare.Backup"
}
New-ItemProperty -Path $BackupExtensionKey -Name "Content Type" -Value "application/vnd.spare.backup+zip" -PropertyType String -Force | Out-Null
New-Item -Path $BackupClass -Force | Out-Null
Set-Item -Path $BackupClass -Value "Spare backup"
New-Item -Path (Join-Path $BackupClass "shell\open\command") -Force | Out-Null
Set-Item -Path (Join-Path $BackupClass "shell\open\command") -Value "`"$(Join-Path $InstallDir 'Spare.exe')`" `"%1`""

Start-Process (Join-Path $InstallDir "Spare.exe")
Write-Output "Spare Desktop was installed in $InstallDir."
Write-Output "The app will initialize its per-user background service automatically."
