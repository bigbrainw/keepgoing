#!/bin/bash
# Render 1200x630 OG image per TASKS.md D2 flat geometry.
set -euo pipefail
cd "$(dirname "$0")"
OUT=img/og.png
mkdir -p img

FONT_600="$(pwd)/fonts/ibm-plex-sans-600.ttf"

swiftc -O -framework AppKit -framework CoreText -o make-og-bin - <<'SWIFT'
import AppKit
import CoreText
import CoreGraphics

let w: CGFloat = 1200, h: CGFloat = 630
let paper = NSColor(calibratedRed: 0.957, green: 0.965, blue: 0.980, alpha: 1) // --paper
let ink = NSColor(calibratedRed: 0.102, green: 0.137, blue: 0.196, alpha: 1)   // --ink
let bolt = NSColor(calibratedRed: 0.784, green: 0.580, blue: 0.102, alpha: 1)   // --bolt #c8941a
let muted = NSColor(calibratedRed: 0.353, green: 0.404, blue: 0.471, alpha: 1)  // --muted
let baseFill = NSColor(calibratedRed: 0.847, green: 0.875, blue: 0.910, alpha: 1) // #d8dfe8
let lidFill = NSColor(calibratedRed: 0.165, green: 0.200, blue: 0.251, alpha: 1)  // #2a3340
let notchFill = NSColor(calibratedRed: 0.910, green: 0.925, blue: 0.945, alpha: 1)

let fontPath = CommandLine.arguments[1]
let outPath = CommandLine.arguments[2]

var err: Unmanaged<CFError>?
CTFontManagerRegisterFontsForURL(URL(fileURLWithPath: fontPath) as CFURL, .process, &err)

let titleFont = NSFont(name: "IBM Plex Sans", size: 84)
    ?? NSFont(name: "IBMPlexSans-SemiBold", size: 84)
    ?? NSFont.systemFont(ofSize: 84, weight: .semibold)
let domainFont = NSFont(name: "IBM Plex Sans", size: 22)
    ?? NSFont.systemFont(ofSize: 22, weight: .regular)

func drawWhiteBolt(center: CGPoint, radius: CGFloat) {
    guard let ctx = NSGraphicsContext.current?.cgContext else { return }
    ctx.saveGState()
    NSColor.white.setFill()
    let s = radius * 0.9
    let path = CGMutablePath()
    path.move(to: CGPoint(x: center.x - s * 0.10, y: center.y + s * 0.42))
    path.addLine(to: CGPoint(x: center.x - s * 0.34, y: center.y - s * 0.04))
    path.addLine(to: CGPoint(x: center.x - s * 0.04, y: center.y - s * 0.04))
    path.addLine(to: CGPoint(x: center.x - s * 0.20, y: center.y - s * 0.42))
    path.addLine(to: CGPoint(x: center.x + s * 0.10, y: center.y - s * 0.42))
    path.addLine(to: CGPoint(x: center.x + s * 0.34, y: center.y + s * 0.04))
    path.addLine(to: CGPoint(x: center.x + s * 0.04, y: center.y + s * 0.04))
    path.addLine(to: CGPoint(x: center.x + s * 0.20, y: center.y + s * 0.42))
    path.closeSubpath()
    ctx.addPath(path)
    ctx.fillPath()
    ctx.restoreGState()
}

let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: Int(w), pixelsHigh: Int(h),
    bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
    colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
rep.size = NSSize(width: w, height: h)
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)

paper.setFill()
NSRect(x: 0, y: 0, width: w, height: h).fill()

// Headline — baselines at y=250 and y=350, x=80, max width 640
let lineAttr: [NSAttributedString.Key: Any] = [
    .font: titleFont,
    .foregroundColor: ink,
    .kern: -1.5
]
// Spec y values are top-down; AppKit origin is bottom-left.
("Close the lid." as NSString).draw(at: NSPoint(x: 80, y: h - 250), withAttributes: lineAttr)
("Your agent keeps going." as NSString).draw(at: NSPoint(x: 80, y: h - 350), withAttributes: lineAttr)

// Domain — (80, 560)
("keepgoing-pi.vercel.app" as NSString).draw(at: NSPoint(x: 80, y: h - 560), withAttributes: [
    .font: domainFont,
    .foregroundColor: muted
])

// Laptop centred at x=990, resting on y=430
let cx: CGFloat = 990
let baseW: CGFloat = 360, baseH: CGFloat = 22
let baseTop: CGFloat = h - 430
let baseRect = CGRect(x: cx - baseW / 2, y: baseTop - baseH, width: baseW, height: baseH)
baseFill.setFill()
NSBezierPath(roundedRect: baseRect, xRadius: 6, yRadius: 6).fill()

// Lighter notch 120×4 centred on front (top) edge of base
let notchW: CGFloat = 120, notchH: CGFloat = 4
notchFill.setFill()
NSBezierPath(roundedRect: CGRect(x: cx - notchW / 2, y: baseTop - notchH,
    width: notchW, height: notchH), xRadius: 1, yRadius: 1).fill()

// Lid 344×12, y=418–430, centred
let lidW: CGFloat = 344, lidH: CGFloat = 12
let lidRect = CGRect(x: cx - lidW / 2, y: h - 430, width: lidW, height: lidH)
lidFill.setFill()
NSBezierPath(roundedRect: lidRect, xRadius: 4, yRadius: 4).fill()

// Hinge glow: 2 px line full lid width at y=430 + soft shadow below
if let ctx = NSGraphicsContext.current?.cgContext {
    ctx.saveGState()
    ctx.setShadow(offset: CGSize(width: 0, height: -5), blur: 10,
                  color: bolt.withAlphaComponent(0.35).cgColor)
    bolt.setFill()
    NSBezierPath(rect: CGRect(x: cx - lidW / 2, y: baseTop - 1, width: lidW, height: 2)).fill()
    ctx.restoreGState()
}

// Bolt badge: r=18 at (990, 380), halo then circle, white bolt
let badgeCenter = CGPoint(x: cx, y: h - 380)
let badgeR: CGFloat = 18
if let ctx = NSGraphicsContext.current?.cgContext {
    ctx.saveGState()
    ctx.setShadow(offset: .zero, blur: 30, color: bolt.withAlphaComponent(0.25).cgColor)
    bolt.setFill()
    NSBezierPath(ovalIn: CGRect(x: badgeCenter.x - badgeR, y: badgeCenter.y - badgeR,
        width: badgeR * 2, height: badgeR * 2)).fill()
    ctx.restoreGState()
}
bolt.setFill()
NSBezierPath(ovalIn: CGRect(x: badgeCenter.x - badgeR, y: badgeCenter.y - badgeR,
    width: badgeR * 2, height: badgeR * 2)).fill()
drawWhiteBolt(center: badgeCenter, radius: badgeR)

NSGraphicsContext.restoreGraphicsState()

guard let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
try png.write(to: URL(fileURLWithPath: outPath))
SWIFT

./make-og-bin "$FONT_600" "$OUT"
rm -f make-og-bin
ls -la "$OUT"
