$ErrorActionPreference = "Stop"

$InstallDir = if ($env:SPARE_INSTALL_DIR) { $env:SPARE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Spare\bin" }
$Spare = Join-Path $InstallDir "spare.exe"

if (Test-Path $Spare) {
  & $Spare uninstall --yes
}

Start-Process powershell -WindowStyle Hidden -ArgumentList @(
  "-NoProfile",
  "-Command",
  "Start-Sleep -Milliseconds 500; Remove-Item -LiteralPath '$InstallDir' -Recurse -Force -ErrorAction SilentlyContinue"
)
Write-Output "Spare was removed. Site source folders were left unchanged."

