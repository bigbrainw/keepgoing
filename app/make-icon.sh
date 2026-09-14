#!/bin/bash
# Renders KeepGoing.app icon: bolt on a rounded dark square. No external deps.
set -euo pipefail
cd "$(dirname "$0")"

PNG=icon-1024.png
ICONSET=icon.iconset
ICNS=AppIcon.icns
SWIFT=make-icon.swift

cat > "$SWIFT" <<'SWIFT'
import AppKit

let size: CGFloat = 1024
let img = NSImage(size: NSSize(width: size, height: size))
img.lockFocus()
guard let ctx = NSGraphicsContext.current?.cgContext else { exit(1) }

let rect = CGRect(x: 0, y: 0, width: size, height: size)
let radius = size * 0.22
let bg = rect.insetBy(dx: size * 0.06, dy: size * 0.06)
let path = CGPath(roundedRect: bg, cornerWidth: radius, cornerHeight: radius, transform: nil)
ctx.addPath(path)
ctx.setFillColor(NSColor(calibratedRed: 0.11, green: 0.12, blue: 0.16, alpha: 1).cgColor)
ctx.fillPath()

let bolt = NSImage(systemSymbolName: "bolt.fill", accessibilityDescription: nil)!
let symbolSize = size * 0.46
let symbolRect = CGRect(x: (size - symbolSize) / 2, y: (size - symbolSize) / 2 - size * 0.02,
                        width: symbolSize, height: symbolSize)
ctx.saveGState()
ctx.addPath(path)
ctx.clip()
NSColor(calibratedRed: 1.0, green: 0.82, blue: 0.18, alpha: 1).setFill()
bolt.draw(in: symbolRect)
ctx.restoreGState()
img.unlockFocus()

guard let tiff = img.tiffRepresentation,
      let rep = NSBitmapImageRep(data: tiff),
      let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
try png.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
SWIFT

swiftc -O -framework AppKit -o make-icon-bin "$SWIFT"
./make-icon-bin "$PNG"
rm -f make-icon-bin "$SWIFT"

rm -rf "$ICONSET"
mkdir -p "$ICONSET"
make_size() { sips -z "$2" "$2" "$PNG" --out "$ICONSET/$1" >/dev/null; }
make_size icon_16x16.png 16
make_size icon_16x16@2x.png 32
make_size icon_32x32.png 32
make_size icon_32x32@2x.png 64
make_size icon_128x128.png 128
make_size icon_128x128@2x.png 256
make_size icon_256x256.png 256
make_size icon_256x256@2x.png 512
make_size icon_512x512.png 512
make_size icon_512x512@2x.png 1024
iconutil -c icns "$ICONSET" -o "$ICNS"
rm -f "$PNG"
echo "built $ICNS"
