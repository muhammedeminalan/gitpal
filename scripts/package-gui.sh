#!/bin/sh
# gitpal masaüstü paketini üretir; gitpal komut satırı aracı her zaman içindedir
# (Terminal düğmesi ve komutların otomatik kurulumu için, DECISIONS A-042).
#
#   scripts/package-gui.sh <gitpal-gui dosyası> <gitpal dosyası> <sürüm> <çıktı klasörü>
#
#   macOS   → gitpal_<sürüm>_macos_<mimari>.dmg        (scripts/package-macos.sh)
#   Linux   → gitpal-gui_<sürüm>_linux_<mimari>.tar.gz  (gitpal-gui + gitpal)
#   Windows → gitpal-gui_<sürüm>_windows_<mimari>.zip   (gitpal-gui.exe + gitpal.exe)
#
# Her paketin yanına "<paket>.sha256" yazılır; uygulamanın güncelleyicisi indirdiğini
# bununla doğrular, tutmazsa dosyayı siler. Üretilen dosyaların yolları basılır.
set -eu

fail() {
	echo "package-gui.sh: $*" >&2
	exit 1
}

[ $# -eq 4 ] || fail "kullanım: package-gui.sh <gitpal-gui> <gitpal> <sürüm> <çıktı klasörü>"
gui=$1
cli=$2
version=${3#v}
out=$4
for f in "$gui" "$cli"; do
	[ -f "$f" ] || fail "dosya yok: $f"
done
arch=${GOARCH:-$(go env GOARCH)}
mkdir -p "$out"
here=$(cd "$(dirname "$0")" && pwd)

case "$(uname -s)" in
Darwin)
	name="gitpal_${version}_macos_${arch}.dmg"
	sh "$here/package-macos.sh" "$gui" "$cli" "$version" "$out/$name" >/dev/null
	;;
Linux)
	name="gitpal-gui_${version}_linux_${arch}.tar.gz"
	stage=$(mktemp -d)
	trap 'rm -rf "$stage"' EXIT
	cp "$gui" "$stage/gitpal-gui"
	cp "$cli" "$stage/gitpal"
	chmod 755 "$stage/gitpal-gui" "$stage/gitpal"
	tar -czf "$out/$name" -C "$stage" gitpal-gui gitpal
	;;
MINGW* | MSYS* | CYGWIN*)
	name="gitpal-gui_${version}_windows_${arch}.zip"
	stage=$(mktemp -d)
	trap 'rm -rf "$stage"' EXIT
	cp "$gui" "$stage/gitpal-gui.exe"
	cp "$cli" "$stage/gitpal.exe"
	# Windows'ta zip için PowerShell'in kendi aracı; ek program gerekmez.
	(cd "$stage" && powershell.exe -NoProfile -Command "Compress-Archive -Path gitpal-gui.exe,gitpal.exe -DestinationPath out.zip -Force")
	mv "$stage/out.zip" "$out/$name"
	;;
*)
	fail "desteklenmeyen sistem: $(uname -s)"
	;;
esac

if command -v sha256sum >/dev/null 2>&1; then
	hash=$(sha256sum "$out/$name" | awk '{ print $1 }')
else
	hash=$(shasum -a 256 "$out/$name" | awk '{ print $1 }')
fi
printf '%s  %s\n' "$hash" "$name" >"$out/$name.sha256"
echo "$out/$name"
echo "$out/$name.sha256"
