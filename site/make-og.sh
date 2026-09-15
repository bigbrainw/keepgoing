#!/bin/bash
# Render 1200x630 OG image matching site tokens (--paper, --bolt, IBM Plex Sans).
set -euo pipefail
cd "$(dirname "$0")"
OUT=img/og.png
mkdir -p img

swiftc -O -framework AppKit -o make-og-bin - <<'SWIFT'
import AppKit

let w: CGFloat = 1200, h: CGFloat = 630
let img = NSImage(size: NSSize(width: w, height: h))
img.lockFocus()

// --paper light scheme (OG previews are usually light)
let paper = NSColor(calibratedRed: 0.957, green: 0.965, blue: 0.980, alpha: 1) // #f4f6fa
let ink = NSColor(calibratedRed: 0.102, green: 0.137, blue: 0.196, alpha: 1)   // #1a2332
let bolt = NSColor(calibratedRed: 0.784, green: 0.580, blue: 0.102, alpha: 1)   // #c8941a
let muted = NSColor(calibratedRed: 0.353, green: 0.404, blue: 0.471, alpha: 1)  // #5a6778

paper.setFill()
NSRect(x: 0, y: 0, width: w, height: h).fill()

let title = "Close the lid. Your agent keeps going."
let domain = "keepgoing-pi.vercel.app"

let titleFont = NSFont(name: "IBMPlexSans-SemiBold", size: 52)
    ?? NSFont.systemFont(ofSize: 52, weight: .semibold)
let domainFont = NSFont(name: "IBMPlexSans-Regular", size: 22)
    ?? NSFont.systemFont(ofSize: 22, weight: .regular)

let tAttr: [NSAttributedString.Key: Any] = [
    .font: titleFont,
    .foregroundColor: ink,
    .kern: -1.0
]
let dAttr: [NSAttributedString.Key: Any] = [
    .font: domainFont,
    .foregroundColor: muted
]

let ts = title as NSString
let ds = domain as NSString
ts.draw(in: NSRect(x: 80, y: h - 220, width: w - 160, height: 140), withAttributes: tAttr)
ds.draw(in: NSRect(x: 80, y: 80, width: w - 160, height: 32), withAttributes: dAttr)

// Gold bolt (matches site .icon-bolt)
if let boltImg = NSImage(systemSymbolName: "bolt.fill", accessibilityDescription: nil) {
    bolt.setFill()
    boltImg.draw(in: NSRect(x: 80, y: h - 310, width: 56, height: 56))
}

img.unlockFocus()
guard let tiff = img.tiffRepresentation,
      let rep = NSBitmapImageRep(data: tiff),
      let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
try png.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
SWIFT

./make-og-bin "$OUT"
rm -f make-og-bin
ls -la "$OUT"
