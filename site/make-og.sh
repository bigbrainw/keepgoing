#!/bin/bash
# Render 1200x630 OG image from hero text. No external services.
set -euo pipefail
cd "$(dirname "$0")"
OUT=img/og.png
mkdir -p img

swiftc -O -framework AppKit -o make-og-bin - <<'SWIFT'
import AppKit

let w: CGFloat = 1200, h: CGFloat = 630
let img = NSImage(size: NSSize(width: w, height: h))
img.lockFocus()
NSColor(calibratedRed: 0.07, green: 0.07, blue: 0.08, alpha: 1).setFill()
NSRect(x: 0, y: 0, width: w, height: h).fill()

let title = "Close the lid. Your agent keeps going."
let sub = "KeepGoing — macOS menu bar app"
let tAttr: [NSAttributedString.Key: Any] = [
    .font: NSFont.systemFont(ofSize: 52, weight: .bold),
    .foregroundColor: NSColor.white,
    .kern: -1.0
]
let sAttr: [NSAttributedString.Key: Any] = [
    .font: NSFont.systemFont(ofSize: 28, weight: .regular),
    .foregroundColor: NSColor(calibratedWhite: 0.65, alpha: 1)
]
let ts = title as NSString
let ss = sub as NSString
ts.draw(in: NSRect(x: 72, y: h - 200, width: w - 144, height: 120), withAttributes: tAttr)
ss.draw(in: NSRect(x: 72, y: h - 260, width: w - 144, height: 40), withAttributes: sAttr)

let bolt = NSImage(systemSymbolName: "bolt.fill", accessibilityDescription: nil)!
NSColor(calibratedRed: 0.96, green: 0.78, blue: 0.26, alpha: 1).setFill()
bolt.draw(in: NSRect(x: 72, y: h - 140, width: 48, height: 48))

img.unlockFocus()
guard let tiff = img.tiffRepresentation,
      let rep = NSBitmapImageRep(data: tiff),
      let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
try png.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
SWIFT

./make-og-bin "$OUT"
rm -f make-og-bin
ls -la "$OUT"
