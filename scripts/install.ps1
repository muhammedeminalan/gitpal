# gitpal kurulum betiği (Windows, PowerShell 5.1+).
#
#   irm https://raw.githubusercontent.com/muhammedeminalan/gitpal/main/scripts/install.ps1 | iex
#
# Ayarlar (ortam değişkeni):
#   GITPAL_VERSION   kurulacak sürüm, örn. 1.0.0 (varsayılan: son sürüm)
#   GITPAL_PREFIX    kurulacak klasör (varsayılan: %LOCALAPPDATA%\Programs\gitpal)
#
# İndirilen arşiv, sürümün checksums.txt dosyasıyla doğrulanır; tutmazsa
# hiçbir şey kurulmaz.
$ErrorActionPreference = 'Stop'

$repo = 'muhammedeminalan/gitpal'
$prefix = if ($env:GITPAL_PREFIX) { $env:GITPAL_PREFIX } else { Join-Path $env:LOCALAPPDATA 'Programs\gitpal' }

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
  Write-Warning "git bulunamadı; gitpal'ın çalışması için git 2.30+ (Git for Windows) gerekir"
}

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'amd64' }
  'ARM64' { 'arm64' }
  default { throw "desteklenmeyen işlemci: $env:PROCESSOR_ARCHITECTURE" }
}

$version = $env:GITPAL_VERSION
if (-not $version) {
  $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
}
$version = $version.TrimStart('v')
$base = if ($env:GITPAL_BASE_URL) { $env:GITPAL_BASE_URL } else { "https://github.com/$repo/releases/download/v$version" }

$archive = "gitpal_${version}_windows_${arch}.zip"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Write-Host "gitpal $version (windows/$arch) indiriliyor…"
  Invoke-WebRequest -UseBasicParsing "$base/$archive" -OutFile (Join-Path $tmp $archive)
  Invoke-WebRequest -UseBasicParsing "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

  $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { ($_ -split '\s+')[1] -eq $archive } | Select-Object -First 1
  if (-not $line) { throw "checksums.txt içinde $archive yok" }
  $want = ($line -split '\s+')[0].ToLower()
  $got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $archive)).Hash.ToLower()
  if ($want -ne $got) { throw 'sağlama toplamı tutmuyor; dosya bozuk ya da değiştirilmiş, kurulmadı' }

  Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath (Join-Path $tmp 'x') -Force
  New-Item -ItemType Directory -Force -Path $prefix | Out-Null
  Copy-Item (Join-Path $tmp 'x\gitpal.exe') (Join-Path $prefix 'gitpal.exe') -Force
  # Kısa ad: gp (Windows'ta sembolik bağ yönetici izni ister; küçük bir yönlendirici yeterli).
  Set-Content -Encoding ASCII -Path (Join-Path $prefix 'gp.cmd') -Value "@echo off`r`n`"%~dp0gitpal.exe`" %*"
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $prefix) {
  [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ";$prefix"), 'User')
  Write-Host "$prefix kullanıcı PATH'ine eklendi; yeni bir terminal aç."
}
Write-Host "Kuruldu: $prefix\gitpal.exe (kısa adı: gp)"
& (Join-Path $prefix 'gitpal.exe') --version
