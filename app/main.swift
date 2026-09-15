// KeepGoing menu bar app. Thin UI over the keepgoing daemon.
import Cocoa
import ServiceManagement

let statusURL = URL(string: "http://127.0.0.1:7777/_status")!
let label = "com.elijah.keepgoing"

struct Status {
    var agents = "?"
    var awake = false
    var online = false
    var always = false
    var lidMode = false
    var lidReady = false
    var sleepDisabled = false
    var screenOffAfter = 0
    var hotspot = ""
    var reachable = false
}

final class App: NSObject, NSApplicationDelegate, NSMenuDelegate {
    var item: NSStatusItem!
    var menu = NSMenu()
    var status = Status()
    var timer: Timer?
    var failCount = 0

    var renderedIcon = ""
    var renderedVersion = ""
    var renderedAgents = ""
    var renderedSleep = ""
    var renderedLid = ""
    var renderedNetwork = ""
    var renderedHotspot = ""
    var renderedAlways = false
    var renderedScreenOff = false
    var renderedLidToggle = false
    var renderedLogin = false
    var renderedStartHidden = true
    var renderedRestartHidden = false

    let versionItem = NSMenuItem()
    let agentsItem = NSMenuItem()
    let sleepItem = NSMenuItem()
    let lidItem = NSMenuItem()
    let netItem = NSMenuItem()
    let hotspotItem = NSMenuItem()
    let lidToggleItem = NSMenuItem(title: "Keep awake with lid closed", action: #selector(toggleLid), keyEquivalent: "")
    let alwaysItem = NSMenuItem(title: "Always keep awake", action: #selector(toggleAlways), keyEquivalent: "")
    let screenOffItem = NSMenuItem(title: "Turn off screen when idle", action: #selector(toggleScreenOff), keyEquivalent: "")
    let hotspotActionItem = NSMenuItem(title: "Set hotspot…", action: #selector(setHotspot), keyEquivalent: "")
    let loginItem = NSMenuItem(title: "Open at login", action: #selector(toggleLogin), keyEquivalent: "")
    let logItem = NSMenuItem(title: "Show log", action: #selector(openLog), keyEquivalent: "l")
    let startDaemonItem = NSMenuItem(title: "Start daemon", action: #selector(startDaemon), keyEquivalent: "")
    let restartItem = NSMenuItem(title: "Restart daemon", action: #selector(restartDaemon), keyEquivalent: "")
    let aboutItem = NSMenuItem(title: "About KeepGoing", action: #selector(showAbout), keyEquivalent: "")

    func applicationDidFinishLaunching(_ n: Notification) {
        item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        item.button?.image = symbol("bolt.slash")
        buildMenu()
        item.menu = menu
        menu.delegate = self
        ensureDaemon()
        showOnboardingIfNeeded()
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { _ in self.refresh() }
        loginItem.state = SMAppService.mainApp.status == .enabled ? .on : .off
        renderedLogin = loginItem.state == .on
    }

    func buildMenu() {
        versionItem.isEnabled = false
        menu.addItem(versionItem)
        menu.addItem(.separator())
        for it in [agentsItem, sleepItem, lidItem, netItem, hotspotItem] {
            it.isEnabled = false
            menu.addItem(it)
        }
        menu.addItem(.separator())
        lidToggleItem.target = self
        alwaysItem.target = self
        screenOffItem.target = self
        hotspotActionItem.target = self
        menu.addItem(lidToggleItem)
        menu.addItem(alwaysItem)
        menu.addItem(screenOffItem)
        menu.addItem(hotspotActionItem)
        menu.addItem(.separator())
        loginItem.target = self
        logItem.target = self
        startDaemonItem.target = self
        restartItem.target = self
        menu.addItem(loginItem)
        menu.addItem(logItem)
        startDaemonItem.isHidden = true
        menu.addItem(startDaemonItem)
        menu.addItem(restartItem)
        menu.addItem(.separator())
        aboutItem.target = self
        menu.addItem(aboutItem)
        let quit = NSMenuItem(title: "Quit KeepGoing", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        menu.addItem(quit)

        let ver = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "?"
        setTitle(versionItem, to: "KeepGoing \(ver)", store: &renderedVersion)
    }

    func menuWillOpen(_ menu: NSMenu) {
        refresh()
    }

    // MARK: state

    func refresh() {
        var req = URLRequest(url: statusURL)
        req.timeoutInterval = 2
        URLSession.shared.dataTask(with: req) { data, _, _ in
            var s = Status()
            if let d = data, let j = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
                s.reachable = true
                s.agents = j["agents"] as? String ?? "?"
                s.awake = j["awake"] as? Bool ?? false
                s.online = j["online"] as? Bool ?? false
                s.always = j["always_awake"] as? Bool ?? false
                s.lidMode = j["lid_mode"] as? Bool ?? false
                s.lidReady = j["lid_ready"] as? Bool ?? false
                s.sleepDisabled = j["sleep_disabled"] as? Bool ?? false
                s.screenOffAfter = j["screen_off_after"] as? Int ?? 0
                s.hotspot = j["hotspot"] as? String ?? ""
            }
            DispatchQueue.main.async {
                if s.reachable {
                    self.failCount = 0
                } else {
                    self.failCount += 1
                }
                self.status = s
                self.render()
            }
        }.resume()
    }

    var daemonUp: Bool {
        status.reachable && failCount < 3
    }

    func render() {
        let s = status
        let up = daemonUp

        let icon = statusIcon(up: up, s: s)
        if icon != renderedIcon {
            item.button?.image = symbol(icon)
            renderedIcon = icon
        }

        if up {
            let ver = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "?"
            setTitle(versionItem, to: "KeepGoing \(ver)", store: &renderedVersion)
            for it in [agentsItem, sleepItem, lidItem, netItem, hotspotItem] { it.isHidden = false }
            setTitle(agentsItem, to: "Agents: \(formatAgents(s.agents))", store: &renderedAgents)
            setTitle(sleepItem, to: s.awake ? "Sleep: blocked" : "Sleep: allowed — no agents", store: &renderedSleep)
            setTitle(lidItem, to: lidStatus(s), store: &renderedLid)
            setTitle(netItem, to: s.online ? "Network: online" : "Network: offline — recovering", store: &renderedNetwork)
            setTitle(hotspotItem, to: s.hotspot.isEmpty ? "Hotspot: not set" : "Hotspot: \(s.hotspot)", store: &renderedHotspot)
            setCheck(lidToggleItem, s.lidMode, store: &renderedLidToggle)
            setCheck(alwaysItem, s.always, store: &renderedAlways)
            setCheck(screenOffItem, s.screenOffAfter > 0, store: &renderedScreenOff)
        } else {
            setTitle(versionItem, to: "Daemon not running", store: &renderedVersion)
            for it in [agentsItem, sleepItem, lidItem, netItem, hotspotItem] { it.isHidden = true }
        }

        let loginOn = SMAppService.mainApp.status == .enabled
        setCheck(loginItem, loginOn, store: &renderedLogin)

        let showStart = !up
        if showStart != !renderedStartHidden {
            startDaemonItem.isHidden = !showStart
            renderedStartHidden = !showStart
        }
        if up != !renderedRestartHidden {
            restartItem.isHidden = !up
            renderedRestartHidden = !up
        }
    }

    func statusIcon(up: Bool, s: Status) -> String {
        if !up { return "bolt.slash" }
        if !s.online { return "wifi.slash" }
        if !s.awake { return "moon.zzz" }
        let lidSafe = s.lidReady && s.sleepDisabled
        return lidSafe ? "bolt.fill" : "bolt"
    }

    func lidStatus(_ s: Status) -> String {
        if s.lidMode && !s.lidReady { return "Lid: setup needed" }
        if s.lidReady && s.sleepDisabled { return "Lid: safe to close" }
        return "Lid: will sleep"
    }

    func formatAgents(_ raw: String) -> String {
        let trimmed = raw.trimmingCharacters(in: .whitespaces)
        if trimmed.isEmpty || trimmed == "?" { return "none" }
        let parts = trimmed.split(separator: " ").map { part -> String in
            var s = String(part).replacingOccurrences(of: "×", with: " ")
            s = s.replacingOccurrences(of: "codex-app", with: "codex")
            return s
        }
        return parts.joined(separator: " · ")
    }

    func setTitle(_ item: NSMenuItem, to title: String, store: inout String) {
        if title != store {
            item.title = title
            store = title
        }
    }

    func setCheck(_ item: NSMenuItem, _ on: Bool, store: inout Bool) {
        let state: NSControl.StateValue = on ? .on : .off
        if on != store || item.state != state {
            item.state = state
            store = on
        }
    }

    func symbol(_ name: String) -> NSImage? {
        guard let img = NSImage(systemSymbolName: name, accessibilityDescription: nil) else { return nil }
        img.isTemplate = true
        return img.withSymbolConfiguration(NSImage.SymbolConfiguration(pointSize: 16, weight: .regular))
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
        let out = Pipe()
        p.standardOutput = out
        p.standardError = out
        if let input = input {
            let inp = Pipe()
            p.standardInput = inp
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
        let (code, _) = run(["/bin/launchctl", "print", "gui/\(getuid())/\(label)"])
        if code != 0 { run([cli, "install"]) }
    }

    @objc func startDaemon() {
        let (code, _) = run(["/bin/launchctl", "print", "gui/\(getuid())/\(label)"])
        if code != 0 {
            run([cli, "install"])
        } else {
            run(["/bin/launchctl", "kickstart", "-k", "gui/\(getuid())/\(label)"])
        }
        failCount = 0
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { self.refresh() }
    }

    @objc func restartDaemon() {
        run(["/bin/launchctl", "kickstart", "-k", "gui/\(getuid())/\(label)"])
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { self.refresh() }
    }

    @objc func toggleAlways() {
        setConfigKey("always_awake", value: !status.always)
        restartDaemon()
    }

    @objc func toggleScreenOff() {
        let enabling = status.screenOffAfter == 0
        run([cli, "screen", "off-after", enabling ? "120" : "0"])
        restartDaemon()
    }

    func configPath() -> String {
        NSHomeDirectory() + "/.config/keepgoing/config.json"
    }

    func onboardedPath() -> String {
        NSHomeDirectory() + "/.config/keepgoing/onboarded"
    }

    func markOnboarded() {
        let path = onboardedPath()
        try? FileManager.default.createDirectory(atPath: (path as NSString).deletingLastPathComponent, withIntermediateDirectories: true)
        FileManager.default.createFile(atPath: path, contents: Data("1\n".utf8), attributes: [.posixPermissions: 0o600])
    }

    func showOnboardingIfNeeded() {
        guard !FileManager.default.fileExists(atPath: onboardedPath()) else { return }
        let alert = NSAlert()
        alert.messageText = "KeepGoing keeps your Mac awake while agents run."
        alert.informativeText = "Lid mode needs your admin password once so the Mac stays awake when the lid is closed."
        alert.addButton(withTitle: "Set up lid mode")
        alert.addButton(withTitle: "Not now")
        NSApp.activate(ignoringOtherApps: true)
        let setup = alert.runModal() == .alertFirstButtonReturn
        markOnboarded()
        if setup { toggleLid() }
    }

    var configUnreadable = false

    func loadConfig() -> [String: Any]? {
        let path = configPath()
        guard FileManager.default.fileExists(atPath: path) else { return [:] }
        guard let d = FileManager.default.contents(atPath: path) else { return nil }
        do {
            let o = try JSONSerialization.jsonObject(with: d)
            guard let j = o as? [String: Any] else { return nil }
            return j
        } catch {
            if !configUnreadable {
                configUnreadable = true
                let alert = NSAlert()
                alert.messageText = "Couldn't read settings"
                alert.informativeText = path
                alert.runModal()
            }
            return nil
        }
    }

    func saveConfig(_ j: [String: Any]) {
        let path = configPath()
        try? FileManager.default.createDirectory(atPath: (path as NSString).deletingLastPathComponent, withIntermediateDirectories: true)
        guard let d = try? JSONSerialization.data(withJSONObject: j, options: .prettyPrinted) else { return }
        FileManager.default.createFile(atPath: path, contents: d, attributes: [.posixPermissions: 0o600])
        let keys = j.keys.sorted().joined(separator: ", ")
        let msg = "[config] wrote \(path) (\(keys))\n"
        if let data = msg.data(using: .utf8) {
            FileHandle.standardError.write(data)
        }
    }

    func setConfigKey(_ key: String, value: Any) {
        guard var j = loadConfig() else { return }
        j[key] = value
        saveConfig(j)
    }

    func showError(_ operation: String, detail: String) {
        let alert = NSAlert()
        alert.messageText = operation
        alert.informativeText = detail.isEmpty ? "Unknown error" : detail
        alert.runModal()
    }

    @objc func toggleLid() {
        let enabling = !status.lidMode
        if enabling && !status.lidReady {
            let alert = NSAlert()
            alert.messageText = "Allow KeepGoing to override lid-closed sleep?"
            alert.informativeText = """
            /usr/bin/pmset -a disablesleep 1
            /usr/bin/pmset -a disablesleep 0
            Asked once. Remove any time with sudo rm /etc/sudoers.d/keepgoing.
            """
            alert.addButton(withTitle: "Install")
            alert.addButton(withTitle: "Cancel")
            NSApp.activate(ignoringOtherApps: true)
            guard alert.runModal() == .alertFirstButtonReturn else {
                refresh()
                return
            }
            let (_, script) = run([cli, "lid", "install-script"])
            guard !script.isEmpty else {
                showError("Couldn't read install script", detail: script)
                refresh()
                return
            }
            let escaped = script
                .replacingOccurrences(of: "\\", with: "\\\\")
                .replacingOccurrences(of: "\"", with: "\\\"")
                .replacingOccurrences(of: "\r\n", with: "\n")
                .replacingOccurrences(of: "\r", with: "\n")
                .replacingOccurrences(of: "\n", with: "\\n")
            var err: NSDictionary?
            let source = "do shell script \"\(escaped)\" with administrator privileges"
            guard let appleScript = NSAppleScript(source: source) else {
                refresh()
                return
            }
            _ = appleScript.executeAndReturnError(&err)
            if let err {
                showError("Couldn't install lid rule", detail: err.description)
                refresh()
                return
            }
            setConfigKey("lid_mode", value: true)
            restartDaemon()
            return
        }
        if enabling {
            setConfigKey("lid_mode", value: true)
            restartDaemon()
            return
        }
        setConfigKey("lid_mode", value: false)
        run([cli, "lid", "disable"])
        restartDaemon()
    }

    @objc func setHotspot() {
        let alert = NSAlert()
        alert.messageText = "Join this hotspot when Wi-Fi is lost"
        alert.informativeText = ""
        alert.addButton(withTitle: "Save")
        alert.addButton(withTitle: "Cancel")
        let box = NSStackView(frame: NSRect(x: 0, y: 0, width: 280, height: 56))
        box.orientation = .vertical
        box.spacing = 6
        let ssid = NSTextField(frame: NSRect(x: 0, y: 0, width: 280, height: 24))
        ssid.placeholderString = "Hotspot name (SSID)"
        ssid.stringValue = status.hotspot
        let pw = NSSecureTextField(frame: NSRect(x: 0, y: 0, width: 280, height: 24))
        pw.placeholderString = "Password"
        box.addArrangedSubview(ssid)
        box.addArrangedSubview(pw)
        alert.accessoryView = box
        NSApp.activate(ignoringOtherApps: true)
        guard alert.runModal() == .alertFirstButtonReturn, !ssid.stringValue.isEmpty, !pw.stringValue.isEmpty else { return }
        let (code, out) = run([cli, "hotspot", "set", ssid.stringValue], input: pw.stringValue)
        if code != 0 {
            showError("Couldn't save hotspot", detail: out)
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
            showError("Couldn't update login item", detail: error.localizedDescription)
        }
        loginItem.state = svc.status == .enabled ? .on : .off
        renderedLogin = loginItem.state == .on
    }

    @objc func showAbout() {
        NSApp.orderFrontStandardAboutPanel(options: [
            .applicationName: "KeepGoing",
            .credits: NSAttributedString(string: "Open source, MIT · github.com/bigbrainw/keepgoing · buymeacoffee.com/TODO_BMC_URL")
        ])
    }
}

let app = NSApplication.shared
let delegate = App()
app.delegate = delegate
app.setActivationPolicy(.accessory)
app.run()
