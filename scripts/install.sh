#!/bin/sh
# gitpal kurulum betiği (macOS ve Linux).
#
#   curl -fsSL https://raw.githubusercontent.com/muhammedeminalan/gitpal/main/scripts/install.sh | sh
#
# Ayarlar (ortam değişkeni):
#   GITPAL_VERSION   kurulacak sürüm, örn. 1.0.0 (varsayılan: son sürüm)
#   GITPAL_PREFIX    kurulacak klasör (varsayılan: ~/.local/bin)
#
# İndirilen arşiv, sürümün checksums.txt dosyasıyla doğrulanır; tutmazsa
# hiçbir şey kurulmaz.
set -eu

repo="muhammedeminalan/gitpal"
prefix="${GITPAL_PREFIX:-$HOME/.local/bin}"
base="${GITPAL_BASE_URL:-}" # yalnızca testler için: sürüm dosyalarının olduğu adres

fail() { echo "gitpal kurulumu: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "'$1' gerekli ama bulunamadı"; }
need curl
need tar
command -v git >/dev/null 2>&1 || echo "uyarı: git bulunamadı; gitpal'ın çalışması için git 2.30+ gerekir" >&2

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "desteklenmeyen işletim sistemi: $(uname -s) (Windows için install.ps1)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "desteklenmeyen işlemci: $(uname -m)" ;;
esac

version="${GITPAL_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
    sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1)
  [ -n "$version" ] || fail "son sürüm bulunamadı; GITPAL_VERSION ile belirt"
fi
version="${version#v}"
[ -n "$base" ] || base="https://github.com/$repo/releases/download/v$version"

archive="gitpal_${version}_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "gitpal $version ($os/$arch) indiriliyor…"
curl -fsSL -o "$tmp/$archive" "$base/$archive" || fail "$archive indirilemedi"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "checksums.txt indirilemedi"

want=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || fail "checksums.txt içinde $archive yok"
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
else
  got=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
fi
[ "$want" = "$got" ] || fail "sağlama toplamı tutmuyor; dosya bozuk ya da değiştirilmiş, kurulmadı"

tar -xzf "$tmp/$archive" -C "$tmp" gitpal
mkdir -p "$prefix"
install -m 0755 "$tmp/gitpal" "$prefix/gitpal"
ln -sf gitpal "$prefix/gp"

echo "Kuruldu: $prefix/gitpal (kısa adı: gp)"
case ":$PATH:" in
  *":$prefix:"*) ;;
  *) echo "Not: $prefix PATH'te değil. Kabuk ayarına ekle: export PATH=\"$prefix:\$PATH\"" ;;
esac
"$prefix/gitpal" --version
