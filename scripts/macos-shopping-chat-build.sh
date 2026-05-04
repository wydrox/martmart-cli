#!/usr/bin/env bash
set -euo pipefail

CONFIG="${CONFIG:-release}"
SIGN_IDENTITY="${SIGN_IDENTITY:--}"

swift build --package-path macos-app -c "$CONFIG"
BIN="macos-app/.build/$CONFIG/MartMartShoppingChat"
APP="macos-app/.build/$CONFIG/MartMartShoppingChat.app"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/MartMartShoppingChat"
if [[ -x ./martmart ]]; then
  cp ./martmart "$APP/Contents/Resources/martmart"
fi
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>MartMart Shopping Chat</string>
  <key>CFBundleDisplayName</key><string>MartMart Shopping Chat</string>
  <key>CFBundleIdentifier</key><string>dev.martmart.shoppingchat</string>
  <key>CFBundleVersion</key><string>0.1</string>
  <key>CFBundleShortVersionString</key><string>0.1</string>
  <key>CFBundleExecutable</key><string>MartMartShoppingChat</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>LSMinimumSystemVersion</key><string>14.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

if command -v codesign >/dev/null 2>&1; then
  codesign --force --sign "$SIGN_IDENTITY" "$BIN"
  codesign --force --sign "$SIGN_IDENTITY" "$APP"
fi

cat <<MSG
Built: $BIN
Bundled app: $APP
Bundled martmart helper: $(test -x "$APP/Contents/Resources/martmart" && echo yes || echo no)

Run:
  open -n "$APP"

Self-check:
  $BIN --self-check
MSG
