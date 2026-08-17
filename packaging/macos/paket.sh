#!/usr/bin/env bash
# Hizmetra Yazıcı — macOS .app bundle + .dmg paketleyici.
#
# release.yml'deki "macos" job'ında (macos-latest) çalışır; lipo/sips/iconutil/
# hdiutil YALNIZ macOS'ta vardır — bu script Windows/Linux'ta ÇALIŞMAZ, orada
# yalnız YAML/mantık incelemesiyle doğrulanır, runtime CI'da (Mac runner) koşar.
#
# Girdi: aynı dizinde daha önce derlenmiş darwin ikilileri
#   hizmetra-kopru_darwin_amd64  ve  hizmetra-kopru_darwin_arm64
# (release.yml'in "Darwin ikililerini derle" adımı üretir).
# Çıktı: dist-macos/HizmetraYazici.dmg  (imzasız — bkz. packaging/macos/BENI-OKU.txt)
#
# Kullanım:  bash packaging/macos/paket.sh <surum> [amd64_ikili] [arm64_ikili]
set -euo pipefail
# Türkçe dosya adları ("Hizmetra Yazıcı.app") için UTF-8 yerel ayar şart:
# GitHub macOS runner'ında LANG bazen ayarsız (C locale) gelir ve non-ASCII
# dosya adlarını bozar. en_US.UTF-8 macOS'ta her zaman kuruludur.
export LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8

VER="${1:?kullanım: paket.sh <surum> [amd64_ikili] [arm64_ikili]}"
AMD64_BIN="${2:-hizmetra-kopru_darwin_amd64}"
ARM64_BIN="${3:-hizmetra-kopru_darwin_arm64}"

# Repo kökü (bu script packaging/macos/ altında).
KOK="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CIKTI="dist-macos"
BUNDLE="Hizmetra Yazıcı.app"
CONTENTS="$CIKTI/$BUNDLE/Contents"

for f in "$AMD64_BIN" "$ARM64_BIN"; do
  [ -f "$f" ] || { echo "HATA: darwin ikilisi bulunamadı: $f" >&2; exit 1; }
done

rm -rf "$CIKTI"
mkdir -p "$CONTENTS/MacOS" "$CONTENTS/Resources"

# 1) Evrensel (universal2) ikili: TEK .app hem Intel (amd64) hem Apple Silicon
#    (arm64) Mac'lerde çalışsın — kafe sahibi mimari seçmek zorunda kalmasın.
echo "==> lipo: evrensel ikili"
lipo -create "$AMD64_BIN" "$ARM64_BIN" -output "$CONTENTS/MacOS/hizmetra-kopru"
chmod +x "$CONTENTS/MacOS/hizmetra-kopru"

# 2) Info.plist — sürüm şablondan doldurulur.
echo "==> Info.plist ($VER)"
sed "s/__VERSION__/${VER}/g" "$KOK/packaging/macos/Info.plist.template" > "$CONTENTS/Info.plist"

# 3) icon.icns — assets/hizmetra.png'den (256px) iconset üret + iconutil.
#    Kaynak 256px olduğu için 512/1024 (@2x üstü) katmanlar ATLANIR; iconutil
#    kısmi iconset'i kabul eder. Daha keskin Retina için kaynak vektör/1024px PNG
#    gerekir (bkz. assets/ikon_uret.py — orijinal 4000px logo repoda değil).
echo "==> icon.icns"
ICONSET="$CIKTI/icon.iconset"
rm -rf "$ICONSET"; mkdir -p "$ICONSET"
SRC_PNG="$KOK/assets/hizmetra.png"
[ -f "$SRC_PNG" ] || { echo "HATA: kaynak PNG yok: $SRC_PNG" >&2; exit 1; }
sips -z 16 16   "$SRC_PNG" --out "$ICONSET/icon_16x16.png"      >/dev/null
sips -z 32 32   "$SRC_PNG" --out "$ICONSET/icon_16x16@2x.png"   >/dev/null
sips -z 32 32   "$SRC_PNG" --out "$ICONSET/icon_32x32.png"      >/dev/null
sips -z 64 64   "$SRC_PNG" --out "$ICONSET/icon_32x32@2x.png"   >/dev/null
sips -z 128 128 "$SRC_PNG" --out "$ICONSET/icon_128x128.png"    >/dev/null
sips -z 256 256 "$SRC_PNG" --out "$ICONSET/icon_128x128@2x.png" >/dev/null
sips -z 256 256 "$SRC_PNG" --out "$ICONSET/icon_256x256.png"    >/dev/null
iconutil -c icns "$ICONSET" -o "$CONTENTS/Resources/icon.icns"

# 4) PkgInfo — Finder için standart (isteğe bağlı ama gelenek).
printf 'APPL????' > "$CONTENTS/PkgInfo"

# 5) .dmg — "Applications'a sürükle" arayüzü: .app + Applications symlink + not.
echo "==> .dmg"
DMGROOT="$CIKTI/dmgroot"
rm -rf "$DMGROOT"; mkdir -p "$DMGROOT"
cp -R "$CIKTI/$BUNDLE" "$DMGROOT/"
ln -s /Applications "$DMGROOT/Applications"
cp "$KOK/packaging/macos/BENI-OKU.txt" "$DMGROOT/BENI-OKU.txt"
hdiutil create -volname "Hizmetra Yazıcı" -srcfolder "$DMGROOT" \
  -ov -format UDZO "$CIKTI/HizmetraYazici.dmg"

echo "==> TAMAM: $CIKTI/HizmetraYazici.dmg"
