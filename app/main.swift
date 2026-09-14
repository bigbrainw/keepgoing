// KeepGoing menu bar app. Thin UI over the keepgoing daemon: shows agents /
// awake / online state, toggles always-awake, sets hotspot, opens the log.
// The daemon itself runs under launchd (installed by the bundled CLI).
import Cocoa
import ServiceManagement

let statusURL = URL(string: "http://127.0.0.1:7777/_status")!
let label = "com.elijah.keepgoing"

struct Status {
    var agents = "?"
    var awake = false
    var online = false
    var always = false
    var hotspot = ""
    var held: Int = 0
    var stateSince = ""
    var reachable = false
}

final class App: NSObject, NSApplicationDelegate {
    var item: NSStatusItem!
    var menu = NSMenu()
    var status = Status()
    var timer: Timer?

    // menu items we update in place
    let agentsItem = NSMenuItem()
    let awakeItem = NSMenuItem()
    let netItem = NSMenuItem()
    let hotspotItem = NSMenuItem()
    let alwaysItem = NSMenuItem(title: "Always keep awake", action: #selector(toggleAlways), keyEquivalent: "")
    let daemonItem = NSMenuItem(title: "Daemon: …", action: nil, keyEquivalent: "")
    let loginItem = NSMenuItem(title: "Open at login", action: #selector(toggleLogin), keyEquivalent: "")

    func applicationDidFinishLaunching(_ n: Notification) {
        item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        item.button?.image = symbol("bolt.slash")
        buildMenu()
        item.menu = menu
        ensureDaemon()
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { _ in self.refresh() }
        loginItem.state = SMAppService.mainApp.status == .enabled ? .on : .off
    }

    func buildMenu() {
        for it in [agentsItem, awakeItem, netItem, hotspotItem] { it.isEnabled = false; menu.addItem(it) }
        menu.addItem(.separator())
        alwaysItem.target = self; menu.addItem(alwaysItem)
        let hs = NSMenuItem(title: "Set hotspot…", action: #selector(setHotspot), keyEquivalent: ""); hs.target = self; menu.addItem(hs)
        menu.addItem(.separator())
        daemonItem.isEnabled = false; menu.addItem(daemonItem)
        let restart = NSMenuItem(title: "Restart daemon", action: #selector(restartDaemon), keyEquivalent: ""); restart.target = self; menu.addItem(restart)
        let log = NSMenuItem(title: "Open log", action: #selector(openLog), keyEquivalent: "l"); log.target = self; menu.addItem(log)
        loginItem.target = self; menu.addItem(loginItem)
        menu.addItem(.separator())
        let quit = NSMenuItem(title: "Quit KeepGoing", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q"); menu.addItem(quit)
    }

    // MARK: state

    func refresh() {
        var req = URLRequest(url: statusURL); req.timeoutInterval = 2
        URLSession.shared.dataTask(with: req) { data, _, _ in
            var s = Status()
            if let d = data, let j = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
                s.reachable = true
                s.agents = j["agents"] as? String ?? "?"
                s.awake = j["awake"] as? Bool ?? false
                s.online = j["online"] as? Bool ?? false
                s.always = j["always_awake"] as? Bool ?? false
                s.hotspot = j["hotspot"] as? String ?? ""
                s.stateSince = j["state_since"] as? String ?? ""
                if let st = j["stats"] as? [String: Any] { s.held = st["held"] as? Int ?? 0 }
            }
            DispatchQueue.main.async { self.status = s; self.render() }
        }.resume()
    }

    func render() {
        let s = status
        guard s.reachable else {
            item.button?.image = symbol("bolt.slash")
            agentsItem.title = "Daemon not running"
            awakeItem.title = ""; netItem.title = ""; hotspotItem.title = ""
            daemonItem.title = "Daemon: stopped"
            return
        }
        item.button?.image = symbol(!s.online ? "wifi.slash" : (s.awake ? "bolt.fill" : "moon.zzz"))
        agentsItem.title = "Agents: \(s.agents)"
        awakeItem.title = s.awake ? "Sleep: blocked" : "Sleep: allowed (no agents)"
        netItem.title = s.online ? "Network: online" : "Network: OFFLINE — recovering…"
        hotspotItem.title = s.hotspot.isEmpty ? "Hotspot: not set" : "Hotspot: \(s.hotspot)"
        alwaysItem.state = s.always ? .on : .off
        daemonItem.title = "Daemon: running · \(s.held) requests held"
    }

    func symbol(_ name: String) -> NSImage? {
        let img = NSImage(systemSymbolName: name, accessibilityDescription: "KeepGoing")
        img?.isTemplate = true
        return img
    }

    // MARK: actions

    var cli: String {
        Bundle.main.bundlePath + "/Contents/MacOS/keepgoing-cli"
    }

    @discardableResult
    func run(_ args: [String], input: String? = nil) -> (Int32, String) {
        let p = Process()
        p.executableURL = URL(fileURLWithPath: args[0])
        p.arguments = Array(args.dropFirst())
        let out = Pipe(); p.standardOutput = out; p.standardError = out
        if let input = input {
            let inp = Pipe(); p.standardInput = inp
            try? p.run()
            inp.fileHandleForWriting.write((input + "\n").data(using: .utf8)!)
            inp.fileHandleForWriting.closeFile()
        } else {
            try? p.run()
        }
        p.waitUntilExit()
        let s = String(data: out.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
        return (p.terminationStatus, s)
    }

    func ensureDaemon() {
        // if launchd job missing, install it using the bundled CLI
        let (code, _) = run(["/bin/launchctl", "print", "gui/\(getuid())/\(label)"])
        if code != 0 { run([cli, "install"]) }
    }

    @objc func restartDaemon() {
        run(["/bin/launchctl", "kickstart", "-k", "gui/\(getuid())/\(label)"])
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { self.refresh() }
    }

    @objc func toggleAlways() {
        let path = NSHomeDirectory() + "/.config/keepgoing/config.json"
        var j: [String: Any] = [:]
        if let d = FileManager.default.contents(atPath: path),
           let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] { j = o }
        j["always_awake"] = !status.always
        try? FileManager.default.createDirectory(atPath: (path as NSString).deletingLastPathComponent, withIntermediateDirectories: true)
        if let d = try? JSONSerialization.data(withJSONObject: j, options: .prettyPrinted) {
            FileManager.default.createFile(atPath: path, contents: d, attributes: [.posixPermissions: 0o600])
        }
        restartDaemon()
    }

    @objc func setHotspot() {
        let alert = NSAlert()
        alert.messageText = "Hotspot fallback"
        alert.informativeText = "When Wi-Fi is gone for 20s+, KeepGoing bounces the radio, then joins this hotspot. Password is stored in your login Keychain."
        alert.addButton(withTitle: "Save"); alert.addButton(withTitle: "Cancel")
        let box = NSStackView(frame: NSRect(x: 0, y: 0, width: 280, height: 56))
        box.orientation = .vertical; box.spacing = 6
        let ssid = NSTextField(frame: NSRect(x: 0, y: 0, width: 280, height: 24)); ssid.placeholderString = "Hotspot name (SSID)"; ssid.stringValue = status.hotspot
        let pw = NSSecureTextField(frame: NSRect(x: 0, y: 0, width: 280, height: 24)); pw.placeholderString = "Password"
        box.addArrangedSubview(ssid); box.addArrangedSubview(pw)
        alert.accessoryView = box
        NSApp.activate(ignoringOtherApps: true)
        guard alert.runModal() == .alertFirstButtonReturn, !ssid.stringValue.isEmpty, !pw.stringValue.isEmpty else { return }
        let (code, out) = run([cli, "hotspot", "set", ssid.stringValue], input: pw.stringValue)
        if code != 0 {
            let e = NSAlert(); e.messageText = "Could not save hotspot"; e.informativeText = out; e.runModal()
        }
        restartDaemon()
    }

    @objc func openLog() {
        NSWorkspace.shared.open(URL(fileURLWithPath: NSHomeDirectory() + "/Library/Logs/keepgoing/daemon.log"))
    }

    @objc func toggleLogin() {
        let svc = SMAppService.mainApp
        do {
            if svc.status == .enabled { try svc.unregister() } else { try svc.register() }
        } catch {
            let e = NSAlert(); e.messageText = "Login item"; e.informativeText = error.localizedDescription; e.runModal()
        }
        loginItem.state = svc.status == .enabled ? .on : .off
    }
}

let app = NSApplication.shared
let delegate = App()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
