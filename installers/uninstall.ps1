$ErrorActionPreference = "Stop"

$InstallDir = if ($env:SPARE_INSTALL_DIR) { $env:SPARE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Spare\bin" }
$Spare = Join-Path $InstallDir "spare.exe"

if (Test-Path $Spare) {
  & $Spare uninstall --yes
}

$ClassesRoot = "HKCU:\Software\Classes"
$ExtensionKey = Join-Path $ClassesRoot ".sp"
if (Test-Path $ExtensionKey) {
  $CurrentAssociation = (Get-Item $ExtensionKey).GetValue("")
  if ($CurrentAssociation -eq "Spare.Recipe") {
    Remove-Item $ExtensionKey -Recurse -Force
  } else {
    $ContentType = (Get-Item $ExtensionKey).GetValue("Content Type")
    if ($ContentType -eq "application/vnd.spare.recipe+zip") {
      Remove-ItemProperty -Path $ExtensionKey -Name "Content Type" -ErrorAction SilentlyContinue
    }
  }
}
$RecipeClass = Join-Path $ClassesRoot "Spare.Recipe"
if (Test-Path $RecipeClass) {
  Remove-Item $RecipeClass -Recurse -Force
}

Start-Process powershell -WindowStyle Hidden -ArgumentList @(
  "-NoProfile",
  "-Command",
  "Start-Sleep -Milliseconds 500; Remove-Item -LiteralPath '$InstallDir' -Recurse -Force -ErrorAction SilentlyContinue"
)
Write-Output "Spare was removed. Site source folders were left unchanged."
