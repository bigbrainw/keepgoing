#!/bin/bash
# Build release artifacts for KeepGoing (zip + dmg). Ad-hoc signed unless
# SIGN_IDENTITY / NOTARY_PROFILE are set.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

VERSION=$(tr -d '[:space:]' < VERSION)
DIST="$ROOT/dist"
STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT

echo "==> building KeepGoing ${VERSION}"
./app/build.sh "$DIST"

APP="$DIST/KeepGoing.app"
ZIP="$DIST/KeepGoing-${VERSION}.zip"
DMG="$DIST/KeepGoing-${VERSION}.dmg"

sign_app() {
  if [[ -n "${SIGN_IDENTITY:-}" ]]; then
    echo "==> signing with Developer ID: $SIGN_IDENTITY"
    codesign --deep --force --options runtime --timestamp -s "$SIGN_IDENTITY" "$APP"
  else
    echo "==> ad-hoc signing (set SIGN_IDENTITY for Developer ID signing)"
    codesign -s - -f --deep "$APP"
  fi
}

notarize_dmg() {
  if [[ -n "${NOTARY_PROFILE:-}" ]]; then
    echo "==> notarizing with profile: $NOTARY_PROFILE"
    xcrun notarytool submit "$DMG" --keychain-profile "$NOTARY_PROFILE" --wait
    xcrun stapler staple "$DMG"
  else
    echo "==> skipping notarization (set NOTARY_PROFILE to notarize)"
  fi
}

sign_app

echo "==> zip"
ditto -c -k --keepParent "$APP" "$ZIP"

echo "==> dmg"
mkdir -p "$STAGE/KeepGoing"
cp -R "$APP" "$STAGE/KeepGoing/"
ln -s /Applications "$STAGE/KeepGoing/Applications"
hdiutil create -volname KeepGoing -srcfolder "$STAGE/KeepGoing" -ov -format UDZO "$DMG" >/dev/null

if [[ -n "${SIGN_IDENTITY:-}" ]]; then
  codesign --force --timestamp -s "$SIGN_IDENTITY" "$DMG" 2>/dev/null || true
fi
notarize_dmg

if [[ -z "${SIGN_IDENTITY:-}" && -z "${NOTARY_PROFILE:-}" ]]; then
  echo ""
  echo "WARNING: artifacts are ad-hoc signed and not notarized."
  echo "Gatekeeper may block on other Macs; see README Install section."
fi

echo ""
echo "SHA256:"
shasum -a 256 "$ZIP" "$DMG"
echo ""
echo "artifacts:"
echo "  $ZIP"
echo "  $DMG"
