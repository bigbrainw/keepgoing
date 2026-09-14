#!/bin/bash
# Builds KeepGoing.app: Swift menu bar UI + bundled Go daemon CLI.
set -euo pipefail
cd "$(dirname "$0")"
OUT=${1:-../dist}
VERSION=$(tr -d '[:space:]' < ../VERSION)
mkdir -p "$OUT/KeepGoing.app/Contents/MacOS" "$OUT/KeepGoing.app/Contents/Resources"
APP=$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")/KeepGoing.app
if [[ ! -f AppIcon.icns ]]; then
  ./make-icon.sh
fi
( cd .. && go build -ldflags="-linkmode=external -X main.version=${VERSION}" -o "$APP/Contents/MacOS/keepgoing-cli" . )
swiftc -O -framework Cocoa -framework ServiceManagement -o "$APP/Contents/MacOS/KeepGoing" main.swift
cp Info.plist "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString ${VERSION}" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion ${VERSION}" "$APP/Contents/Info.plist"
cp AppIcon.icns "$APP/Contents/Resources/AppIcon.icns"
codesign -s - -f --deep "$APP" 2>/dev/null
echo "built $APP (${VERSION})"
