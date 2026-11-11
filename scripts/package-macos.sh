#!/bin/sh
# gitpal masaüstü arayüzünü macOS için paketler: derlenmiş gitpal-gui dosyasını
# gitpal.app içine koyar ve onu bir .dmg disk imajına sarar.
#
#   scripts/package-macos.sh <gitpal-gui dosyası> <sürüm> <çıktı.dmg>
#   scripts/package-macos.sh bin/gitpal-gui 1.0.0 dist/gitpal_1.0.0_macos_arm64.dmg
#
# Yalnızca macOS'ta çalışır (codesign ve hdiutil gerekir). Kolay yolu: `make dmg`.
#
# İmza: uygulama "ad-hoc" imzalanır. Apple Silicon imzasız kodu hiç çalıştırmaz ve
# imzası Info.plist'i kapsamayan bir paketi "hasarlı" sayar; ad-hoc imza ikisini de
# çözer. Apple'ın geliştirici imzası ve notarization YOKTUR: indirilen uygulama ilk
# açılışta Gatekeeper uyarısı verir (Sistem Ayarları → Gizlilik ve Güvenlik →
# "Yine de Aç"). Bkz. DECISIONS A-039.
set -eu

fail() {
	echo "package-macos.sh: $*" >&2
	exit 1
}

[ "$(uname -s)" = "Darwin" ] || fail "yalnızca macOS'ta çalışır"
[ $# -eq 3 ] || fail "kullanım: package-macos.sh <gitpal-gui dosyası> <sürüm> <çıktı.dmg>"

binary=$1
version=${2#v}
output=$3

[ -f "$binary" ] || fail "dosya yok: $binary"
[ -x "$binary" ] || fail "çalıştırılabilir değil: $binary"
case "$output" in
*.dmg) ;;
*) fail "çıktı .dmg ile bitmeli: $output" ;;
esac

# Info.plist sürüm alanları yalnızca "1.2.3" biçimini kabul eder; geliştirme
# derlemelerinde (ör. "fbe1345-dirty") 0.0.0 yazılır, gerçek sürüm ayrı bir alanda kalır.
case "$version" in
*[!0-9.]* | "" | .* | *. | *..*) plist_version=0.0.0 ;;
*) plist_version=$version ;;
esac

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM

app="$work/root/gitpal.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$binary" "$app/Contents/MacOS/gitpal-gui"
chmod 755 "$app/Contents/MacOS/gitpal-gui"

cat >"$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>gitpal</string>
	<key>CFBundleDisplayName</key>
	<string>gitpal</string>
	<key>CFBundleIdentifier</key>
	<string>io.github.muhammedeminalan.gitpal</string>
	<key>CFBundleExecutable</key>
	<string>gitpal-gui</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleShortVersionString</key>
	<string>$plist_version</string>
	<key>CFBundleVersion</key>
	<string>$plist_version</string>
	<key>GitpalVersion</key>
	<string>$version</string>
	<key>LSMinimumSystemVersion</key>
	<string>12.0</string>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.developer-tools</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSSupportsAutomaticGraphicsSwitching</key>
	<true/>
</dict>
</plist>
EOF

plutil -lint "$app/Contents/Info.plist" >/dev/null || fail "Info.plist geçersiz"
codesign --force --deep --sign - "$app" 2>/dev/null || fail "codesign başarısız"
codesign --verify --deep --strict "$app" || fail "imza doğrulanamadı"

# Sürükle-bırak kurulum için: pencerede uygulamanın yanında Applications kısayolu.
ln -s /Applications "$work/root/Applications"

mkdir -p "$(dirname "$output")"
rm -f "$output"
# hdiutil yeni macOS sürümlerinde "deprecated" uyarısı basıyor ama çalışıyor; yerine önerilen
# `diskutil image` eski macOS'larda (CI imajı dahil) yok. Çıktısı yalnızca hata olursa gösterilir.
if ! hdiutil create -volname "gitpal $version" -srcfolder "$work/root" \
	-fs HFS+ -format UDZO -ov "$output" >"$work/hdiutil.log" 2>&1; then
	cat "$work/hdiutil.log" >&2
	fail "hdiutil create başarısız"
fi
hdiutil verify "$output" >/dev/null 2>&1 || fail "disk imajı doğrulanamadı"

echo "$output"
