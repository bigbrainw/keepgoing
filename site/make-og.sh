#!/bin/bash
# Render 1200x630 OG image: Plex headline, closed-laptop scene, gold bolt.
set -euo pipefail
cd "$(dirname "$0")"
OUT=img/og.png
mkdir -p img

FONT_600="$(pwd)/fonts/ibm-plex-sans-600.ttf"

swiftc -O -framework AppKit -framework CoreText -o make-og-bin - <<SWIFT
import AppKit
import CoreText
import CoreGraphics

let w: CGFloat = 1200, h: CGFloat = 630
let paper = NSColor(calibratedRed: 0.957, green: 0.965, blue: 0.980, alpha: 1)
let ink = NSColor(calibratedRed: 0.102, green: 0.137, blue: 0.196, alpha: 1)
let bolt = NSColor(calibratedRed: 0.784, green: 0.580, blue: 0.102, alpha: 1)
let muted = NSColor(calibratedRed: 0.353, green: 0.404, blue: 0.471, alpha: 1)
let laptopBase = NSColor(calibratedRed: 0.831, green: 0.847, blue: 0.871, alpha: 1)
let laptopLid = NSColor(calibratedRed: 0.165, green: 0.180, blue: 0.212, alpha: 1)
let closedPanelTop = NSColor(calibratedRed: 0.290, green: 0.290, blue: 0.306, alpha: 1)
let closedPanelBot = NSColor(calibratedRed: 0.227, green: 0.227, blue: 0.243, alpha: 1)

let fontPath = CommandLine.arguments[1]
let outPath = CommandLine.arguments[2]

var err: Unmanaged<CFError>?
CTFontManagerRegisterFontsForURL(URL(fileURLWithPath: fontPath) as CFURL, .process, &err)

let titleFont = NSFont(name: "IBM Plex Sans", size: 96)
    ?? NSFont(name: "IBMPlexSans-SemiBold", size: 96)
    ?? NSFont.systemFont(ofSize: 96, weight: .semibold)
let domainFont = NSFont(name: "IBM Plex Sans", size: 22)
    ?? NSFont.systemFont(ofSize: 22, weight: .regular)

func drawBolt(in rect: CGRect, color: NSColor, fill: NSColor) {
    guard let ctx = NSGraphicsContext.current?.cgContext else { return }
    ctx.saveGState()
    color.setFill()
    let cx = rect.midX, cy = rect.midY, s = min(rect.width, rect.height) * 0.42
    let path = CGMutablePath()
    path.move(to: CGPoint(x: cx - s * 0.12, y: cy + s * 0.55))
    path.addLine(to: CGPoint(x: cx - s * 0.38, y: cy - s * 0.05))
    path.addLine(to: CGPoint(x: cx - s * 0.05, y: cy - s * 0.05))
    path.addLine(to: CGPoint(x: cx - s * 0.22, y: cy - s * 0.55))
    path.addLine(to: CGPoint(x: cx + s * 0.12, y: cy - s * 0.55))
    path.addLine(to: CGPoint(x: cx + s * 0.38, y: cy + s * 0.05))
    path.addLine(to: CGPoint(x: cx + s * 0.05, y: cy + s * 0.05))
    path.addLine(to: CGPoint(x: cx + s * 0.22, y: cy + s * 0.55))
    path.closeSubpath()
    ctx.addPath(path)
    ctx.fillPath()
    ctx.restoreGState()
}

func drawClosedLaptop(origin: CGPoint, scale: CGFloat) {
    guard let ctx = NSGraphicsContext.current?.cgContext else { return }
    ctx.saveGState()
    ctx.translateBy(x: origin.x, y: origin.y)
    ctx.scaleBy(x: scale, y: scale)

    let lw: CGFloat = 320
    let baseH: CGFloat = 54
    let lidH: CGFloat = 170
    let ox = -lw / 2

    // Base
    let baseRect = CGRect(x: ox, y: 0, width: lw, height: baseH)
    laptopBase.setFill()
    let basePath = NSBezierPath(roundedRect: baseRect, xRadius: 10, yRadius: 10)
    basePath.fill()

    // Keyboard dots
    NSColor(calibratedWhite: 0, alpha: 0.12).setFill()
    for row in 0..<3 {
        for col in 0..<28 {
            let x = ox + 22 + CGFloat(col) * 9.5 + (row == 1 ? 4.75 : 0)
            let y = baseH - 28 + CGFloat(row) * 7
            NSBezierPath(ovalIn: CGRect(x: x, y: y, width: 2, height: 2)).fill()
        }
    }

    // Trackpad
    NSColor(calibratedWhite: 0, alpha: 0.14).setStroke()
    let tp = NSBezierPath(roundedRect: CGRect(x: ox + lw/2 - 34, y: 10, width: 68, height: 36), xRadius: 5, yRadius: 5)
    tp.lineWidth = 1
    tp.stroke()

    // Hinge notch on base
    NSColor(calibratedWhite: 0.63, alpha: 1).setFill()
    NSBezierPath(roundedRect: CGRect(x: ox + lw/2 - 26, y: baseH - 4, width: 52, height: 4), xRadius: 2, yRadius: 2).fill()

    // Closed lid (folded nearly flat toward viewer, like demo rotateX(-82deg))
    ctx.saveGState()
    ctx.translateBy(x: ox + lw / 2, y: baseH + 2)
    ctx.rotate(by: -1.05)

    let bezel = CGRect(x: -lw / 2, y: -lidH + 18, width: lw, height: lidH)
    laptopLid.setFill()
    NSBezierPath(roundedRect: bezel, xRadius: 10, yRadius: 10).fill()

    let panel = CGRect(x: -lw / 2 + 8, y: -lidH + 30, width: lw - 16, height: lidH - 36)
    let grad = NSGradient(colors: [closedPanelTop, closedPanelBot])
    grad?.draw(in: panel, angle: 145)

    NSColor.black.setFill()
    NSBezierPath(roundedRect: CGRect(x: -18, y: -lidH + 24, width: 36, height: 5), xRadius: 2, yRadius: 2).fill()

    ctx.restoreGState()

    // Hinge glow
    ctx.saveGState()
    ctx.setShadow(offset: .zero, blur: 12, color: bolt.withAlphaComponent(0.9).cgColor)
    bolt.setFill()
    NSBezierPath(rect: CGRect(x: ox + lw/2 - 55, y: baseH - 2, width: 110, height: 3)).fill()
    ctx.restoreGState()

    // Bolt badge
    let badge = CGRect(x: ox + lw/2 - 14, y: baseH + 10, width: 28, height: 28)
    bolt.setFill()
    NSBezierPath(ovalIn: badge).fill()
    drawBolt(in: badge, color: paper, fill: paper)

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

let line1 = "Close the lid."
let line2 = "Your agent keeps going."
let lineAttr: [NSAttributedString.Key: Any] = [
    .font: titleFont,
    .foregroundColor: ink,
    .kern: -1.8
]
let l1 = line1 as NSString
let l2 = line2 as NSString
let l1Size = l1.size(withAttributes: lineAttr)
let l2Size = l2.size(withAttributes: lineAttr)
let blockH = l1Size.height + l2Size.height + 8
let textY = (h - blockH) / 2
l1.draw(at: NSPoint(x: 80, y: textY + l2Size.height + 8), withAttributes: lineAttr)
l2.draw(at: NSPoint(x: 80, y: textY), withAttributes: lineAttr)

let domain = "keepgoing-pi.vercel.app" as NSString
domain.draw(at: NSPoint(x: 80, y: 56), withAttributes: [
    .font: domainFont,
    .foregroundColor: muted
])

drawClosedLaptop(origin: CGPoint(x: 980, y: 155), scale: 1.0)

NSGraphicsContext.restoreGraphicsState()

guard let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
try png.write(to: URL(fileURLWithPath: outPath))
SWIFT

./make-og-bin "$FONT_600" "$OUT"
rm -f make-og-bin
ls -la "$OUT"
