param(
  [switch]$FromApp,
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

if (-not $FromApp) {
  $Spare = Join-Path $InstallDir "bin\spare.exe"
  if (Test-Path $Spare) {
    & $Spare uninstall --yes
  }
}

schtasks /Delete /TN "Spare Desktop" /F 2>$null | Out-Null

$ClassesRoot = "HKCU:\Software\Classes"
$ExtensionKey = Join-Path $ClassesRoot ".sp"
if (Test-Path $ExtensionKey) {
  $CurrentAssociation = (Get-Item $ExtensionKey).GetValue("")
  if ($CurrentAssociation -eq "Spare.Recipe") {
    Remove-Item $ExtensionKey -Recurse -Force
  }
}
$RecipeClass = Join-Path $ClassesRoot "Spare.Recipe"
if (Test-Path $RecipeClass) {
  Remove-Item $RecipeClass -Recurse -Force
}
$BackupExtensionKey = Join-Path $ClassesRoot ".spare-backup"
if (Test-Path $BackupExtensionKey) {
  $CurrentBackupAssociation = (Get-Item $BackupExtensionKey).GetValue("")
  if ($CurrentBackupAssociation -eq "Spare.Backup") {
    Remove-Item $BackupExtensionKey -Recurse -Force
  }
}
$BackupClass = Join-Path $ClassesRoot "Spare.Backup"
if (Test-Path $BackupClass) {
  Remove-Item $BackupClass -Recurse -Force
}

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$BinDir = Join-Path $InstallDir "bin"
$FilteredPath = @($UserPath -split ";" | Where-Object {
  $_ -and $_.TrimEnd("\") -ne $BinDir.TrimEnd("\")
}) -join ";"
[Environment]::SetEnvironmentVariable("Path", $FilteredPath, "User")

$EscapedInstallDir = $InstallDir.Replace("'", "''")
Start-Process powershell.exe -WindowStyle Hidden -ArgumentList @(
  "-NoProfile",
  "-Command",
  "Start-Sleep -Milliseconds 800; Remove-Item -LiteralPath '$EscapedInstallDir' -Recurse -Force -ErrorAction SilentlyContinue"
)
Write-Output "Spare Desktop was removed. Recipe folders and received files were left unchanged."
