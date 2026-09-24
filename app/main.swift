// KeepGoing menu bar app. Thin UI over the keepgoing daemon.
import Cocoa
import CoreLocation
import CoreWLAN
import UserNotifications

let hotspotGuidance = "iPhone: Settings → Personal Hotspot → Allow Others to Join. Mac: System Settings → Wi-Fi → Ask to join hotspots → Automatically. The name must match your iPhone's name exactly."

final class WiFiManager: NSObject, CLLocationManagerDelegate {
    let loc = CLLocationManager()
    var lastHandledID = 0

    func locationLabel() -> String {
        switch loc.authorizationStatus {
        case .authorizedAlways, .authorizedWhenInUse: return "authorized"
        case .denied, .restricted: return "denied"
        case .notDetermined: return "not determined"
        @unknown default: return "unknown"
        }
    }

    func ensureLocation() {
        if loc.authorizationStatus == .notDetermined {
            loc.delegate = self
            loc.requestWhenInUseAuthorization()
        }
    }

    func handleRequest(_ req: [String: Any], cli: String) {
        let id = req["id"] as? Int ?? 0
        guard id > 0, id != lastHandledID else { return }
        lastHandledID = id
        ensureLocation()
        let joinSSID = req["join"] as? String
        let scanSSID = req["scan"] as? String
        let ssid = joinSSID ?? scanSSID ?? ""
        guard !ssid.isEmpty else { return }
        let doJoin = joinSSID != nil
        DispatchQueue.global(qos: .userInitiated).async {
            self.run(ssid: ssid, id: id, join: doJoin, cli: cli)
        }
    }

    func run(ssid: String, id: Int, join: Bool, cli: String) {
        let locState = locationLabel()
        guard let iface = CWWiFiClient.shared().interface() else {
            post(id: id, ok: false, error: "no Wi-Fi interface", visible: false, rssi: 0, location: locState)
            return
        }
        var match: CWNetwork?
        var rssi = 0
        do {
            let nets = try iface.scanForNetworks(withName: ssid)
            for net in nets where net.ssid == ssid {
                match = net
                rssi = net.rssiValue
                break
            }
        } catch {
            post(id: id, ok: false, error: error.localizedDescription, visible: false, rssi: 0, location: locState)
            return
        }
        guard let network = match else {
            post(id: id, ok: false, error: "not visible", visible: false, rssi: 0, location: locState)
            return
        }
        if !join {
            post(id: id, ok: true, error: "", visible: true, rssi: rssi, location: locState)
            return
        }
        let p = Process()
        p.executableURL = URL(fileURLWithPath: cli)
        p.arguments = ["hotspot", "password"]
        let out = Pipe()
        p.standardOutput = out
        p.standardError = out
        var pw = ""
        do {
            try p.run()
            p.waitUntilExit()
            pw = String(data: out.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
            pw = pw.trimmingCharacters(in: .whitespacesAndNewlines)
        } catch {
            post(id: id, ok: false, error: error.localizedDescription, visible: true, rssi: rssi, location: locState)
            return
        }
        do {
            try iface.associate(to: network, password: pw)
            post(id: id, ok: true, error: "", visible: true, rssi: rssi, location: locState)
        } catch {
            post(id: id, ok: false, error: error.localizedDescription, visible: true, rssi: rssi, location: locState)
        }
    }

    func post(id: Int, ok: Bool, error: String, visible: Bool, rssi: Int, location: String) {
        var body: [String: Any] = ["id": id, "ok": ok, "visible": visible, "rssi": rssi, "location": location]
        if !error.isEmpty { body["error"] = error }
        var req = URLRequest(url: URL(string: "http://127.0.0.1:7777/_wifi")!)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try? JSONSerialization.data(withJSONObject: body)
        URLSession.shared.dataTask(with: req).resume()
    }
}

let statusURL = URL(string: "http://127.0.0.1:7777/_status")!
let daemonLabel = "com.elijah.keepgoing"

struct Status {
    var agents = "?"
    var awake = false
    var online = false
    var always = false
    var lidMode = false
    var lidReady = false
    var sleepDisabled = false
    var screenOffAfter = 0
    var idleSleepAfter = 0
    var hotspot = ""
    var thermal = ""
    var cpuC: Double?
    var nightMode = ""
    var nightAnswer = ""
    var nightUntilAt = ""
    var nightAskAt = ""
    var primeAt: [String] = []
    var primeNextAt = ""
    var primeLastOK: String = ""
    var reachable = false
}

final class App: NSObject, NSApplicationDelegate, NSMenuDelegate, UNUserNotificationCenterDelegate {
    var item: NSStatusItem!
    var menu = NSMenu()
    var status = Status()
    var timer: Timer?
    var failCount = 0
    var exitReason = "unknown"
    let wifi = WiFiManager()

    var renderedIcon = ""
    var renderedVersion = ""
    var renderedAgents = ""
    var renderedSleep = ""
    var renderedLid = ""
    var renderedNetwork = ""
    var renderedHotspot = ""
    var renderedThermal = ""
    var renderedNight = ""
    var renderedWindow = ""
    var renderedOvernight = false
    var renderedAlways = false
    var renderedScreenOff = false
    var renderedIdleSleep = false
    var renderedLidToggle = false
    var renderedLogin = false
    var renderedStartHidden = true
    var renderedRestartHidden = false
    var thermalNotified = false
    var thermalTimer: Timer?
    var lastNightRequestID = 0
    var nightMenuFallback = false

    let versionItem = NSMenuItem()
    let agentsItem = NSMenuItem()
    let sleepItem = NSMenuItem()
    let lidItem = NSMenuItem()
    let netItem = NSMenuItem()
    let hotspotItem = NSMenuItem()
    let thermalItem = NSMenuItem()
    let nightItem = NSMenuItem()
    let windowItem = NSMenuItem()
    let nightYesItem = NSMenuItem(title: "Keep running overnight", action: #selector(nightAnswerYes), keyEquivalent: "")
    let nightNoItem = NSMenuItem(title: "Let it sleep tonight", action: #selector(nightAnswerNo), keyEquivalent: "")
    let overnightItem = NSMenuItem(title: "Run overnight tonight", action: #selector(toggleOvernight), keyEquivalent: "")
    let lidToggleItem = NSMenuItem(title: "Keep awake with lid closed", action: #selector(toggleLid), keyEquivalent: "")
    let alwaysItem = NSMenuItem(title: "Always keep awake", action: #selector(toggleAlways), keyEquivalent: "")
    let screenOffItem = NSMenuItem(title: "Turn off screen when idle", action: #selector(toggleScreenOff), keyEquivalent: "")
    let idleSleepItem = NSMenuItem(title: "Allow sleep when agents are idle 30 min", action: #selector(toggleIdleSleep), keyEquivalent: "")
    let hotspotActionItem = NSMenuItem(title: "Set hotspot…", action: #selector(setHotspot), keyEquivalent: "")
    let loginItem = NSMenuItem(title: "Open at login", action: #selector(toggleLogin), keyEquivalent: "")
    let logItem = NSMenuItem(title: "Show log", action: #selector(openLog), keyEquivalent: "l")
    let thermalLogItem = NSMenuItem(title: "Show thermal log", action: #selector(openThermalLog), keyEquivalent: "")
    let startDaemonItem = NSMenuItem(title: "Start daemon", action: #selector(startDaemon), keyEquivalent: "")
    let restartItem = NSMenuItem(title: "Restart daemon", action: #selector(restartDaemon), keyEquivalent: "")
    let aboutItem = NSMenuItem(title: "About KeepGoing", action: #selector(showAbout), keyEquivalent: "")

    func applicationDidFinishLaunching(_ n: Notification) {
        appLog("started pid=\(getpid())")
        item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        item.button?.image = symbol("bolt.slash")
        buildMenu()
        item.menu = menu
        menu.delegate = self
        UNUserNotificationCenter.current().delegate = self
        registerNightCategory()
        ensureDaemon()
        showOnboardingIfNeeded()
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 3, repeats: true) { _ in self.refresh() }
        startThermalReporting()
        loginItem.state = appLoginEnabled() ? .on : .off
        renderedLogin = loginItem.state == .on
    }

    func buildMenu() {
        versionItem.isEnabled = false
        menu.addItem(versionItem)
        menu.addItem(.separator())
        for it in [agentsItem, sleepItem, lidItem, netItem, hotspotItem, thermalItem, nightItem, windowItem] {
            it.isEnabled = false
            menu.addItem(it)
        }
        menu.addItem(.separator())
        nightYesItem.target = self
        nightNoItem.target = self
        nightYesItem.isHidden = true
        nightNoItem.isHidden = true
        nightItem.isHidden = true
        lidToggleItem.target = self
        alwaysItem.target = self
        screenOffItem.target = self
        idleSleepItem.target = self
        hotspotActionItem.target = self
        overnightItem.target = self
        menu.addItem(lidToggleItem)
        menu.addItem(alwaysItem)
        menu.addItem(screenOffItem)
        menu.addItem(idleSleepItem)
        menu.addItem(overnightItem)
        menu.addItem(hotspotActionItem)
        menu.addItem(nightYesItem)
        menu.addItem(nightNoItem)
        menu.addItem(.separator())
        loginItem.target = self
        logItem.target = self
        thermalLogItem.target = self
        startDaemonItem.target = self
        restartItem.target = self
        menu.addItem(loginItem)
        menu.addItem(logItem)
        menu.addItem(thermalLogItem)
        startDaemonItem.isHidden = true
        menu.addItem(startDaemonItem)
        menu.addItem(restartItem)
        menu.addItem(.separator())
        aboutItem.target = self
        menu.addItem(aboutItem)
        let quit = NSMenuItem(title: "Quit KeepGoing (stop autostart)", action: #selector(quitStopAutostart), keyEquivalent: "q")
        quit.target = self
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
                s.idleSleepAfter = j["idle_sleep_after"] as? Int ?? 0
                s.hotspot = j["hotspot"] as? String ?? ""
                s.thermal = j["thermal"] as? String ?? ""
                s.cpuC = j["cpu_c"] as? Double
                s.nightMode = j["night_mode"] as? String ?? ""
                if let na = j["night_answer"] as? [String: Any] {
                    s.nightAnswer = na["answer"] as? String ?? ""
                }
                s.nightUntilAt = j["night_until_at"] as? String ?? ""
                s.nightAskAt = j["night_ask_at"] as? String ?? ""
                if let at = j["prime_at"] as? [String] {
                    s.primeAt = at
                } else if let atAny = j["prime_at"] as? [Any] {
                    s.primeAt = atAny.compactMap { $0 as? String }
                }
                s.primeNextAt = j["prime_next_at"] as? String ?? ""
                if let pl = j["prime_last"] as? [String: Any] {
                    s.primeLastOK = latestPrimeTime(pl)
                }
                if let wr = j["wifi_request"] as? [String: Any] {
                    self.wifi.handleRequest(wr, cli: self.cli)
                }
                if let nr = j["night_request"] as? [String: Any] {
                    let id = nr["id"] as? Int ?? 0
                    DispatchQueue.main.async {
                        self.handleNightRequest(id: id, mode: s.nightMode)
                    }
                } else if s.nightMode != "ask" {
                    DispatchQueue.main.async { self.clearNightFallback() }
                }
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

        let icon = nightMenuFallback ? "moon.zzz" : statusIcon(up: up, s: s)
        if icon != renderedIcon {
            item.button?.image = symbol(icon)
            renderedIcon = icon
        }

        if up {
            let ver = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "?"
            setTitle(versionItem, to: "KeepGoing \(ver)", store: &renderedVersion)
            for it in [agentsItem, sleepItem, lidItem, netItem, hotspotItem, thermalItem, nightItem, windowItem] { it.isHidden = false }
            nightItem.isHidden = nightMenuFallback || s.nightMode == "off"
            setTitle(agentsItem, to: "Agents: \(formatAgents(s.agents))", store: &renderedAgents)
            setTitle(sleepItem, to: s.awake ? "Sleep: blocked" : "Sleep: allowed — no agents", store: &renderedSleep)
            setTitle(lidItem, to: lidStatus(s), store: &renderedLid)
            setTitle(netItem, to: s.online ? "Network: online" : "Network: offline — recovering", store: &renderedNetwork)
            setTitle(hotspotItem, to: s.hotspot.isEmpty ? "Hotspot: not set" : "Hotspot: \(s.hotspot)", store: &renderedHotspot)
            setTitle(thermalItem, to: formatThermal(s), store: &renderedThermal)
            setTitle(nightItem, to: nightStatusLine(s), store: &renderedNight)
            setTitle(windowItem, to: windowStatusLine(s), store: &renderedWindow)
            checkThermalNotify(s.thermal)
            setCheck(lidToggleItem, s.lidMode, store: &renderedLidToggle)
            setCheck(alwaysItem, s.always, store: &renderedAlways)
            setCheck(screenOffItem, s.screenOffAfter > 0, store: &renderedScreenOff)
            setCheck(idleSleepItem, s.idleSleepAfter > 0, store: &renderedIdleSleep)
            setCheck(overnightItem, s.nightAnswer == "yes", store: &renderedOvernight)
        } else {
            setTitle(versionItem, to: "Daemon not running", store: &renderedVersion)
            for it in [agentsItem, sleepItem, lidItem, netItem, hotspotItem, thermalItem, nightItem, windowItem] { it.isHidden = true }
        }

        let loginOn = appLoginEnabled()
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
        var s = trimmed.replacingOccurrences(of: "codex-app", with: "codex")
        if s.count > 40 {
            s = String(s.prefix(37)) + "..."
        }
        return s
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

    func formatThermal(_ s: Status) -> String {
        let state = s.thermal.isEmpty ? "—" : s.thermal
        if let c = s.cpuC {
            return String(format: "Thermal: %@ · %.0f °C", state, c)
        }
        return "Thermal: \(state)"
    }

    func thermalName() -> String {
        switch ProcessInfo.processInfo.thermalState {
        case .nominal: return "nominal"
        case .fair: return "fair"
        case .serious: return "serious"
        case .critical: return "critical"
        @unknown default: return "nominal"
        }
    }

    func checkThermalNotify(_ state: String) {
        if state == "serious" || state == "critical" {
            if !thermalNotified {
                postThermalNotification()
                thermalNotified = true
            }
        } else {
            thermalNotified = false
        }
    }

    func startThermalReporting() {
        // Optional fallback: POST ProcessInfo thermal until daemon owns readings.
        reportThermal()
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(thermalChanged),
            name: ProcessInfo.thermalStateDidChangeNotification,
            object: nil
        )
        thermalTimer = Timer.scheduledTimer(withTimeInterval: 10, repeats: true) { _ in self.reportThermal() }
    }

    @objc func thermalChanged() {
        reportThermal()
    }

    func reportThermal() {
        let state = thermalName()
        let body: [String: Any] = ["state": state]
        var req = URLRequest(url: URL(string: "http://127.0.0.1:7777/_thermal")!)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try? JSONSerialization.data(withJSONObject: body)
        URLSession.shared.dataTask(with: req).resume()
    }

    func nightStatusLine(_ s: Status) -> String {
        if nightMenuFallback && s.nightMode == "ask" {
            return "Overnight: asking — choose below"
        }
        switch s.nightMode {
        case "run":
            if let t = formatUntilClock(s.nightUntilAt) {
                return "Overnight: on until \(t)"
            }
            return "Overnight: on until morning"
        case "sleep":
            return "Overnight: off tonight"
        case "ask":
            return "Overnight: asking — choose below"
        case "off":
            if s.nightAskAt.isEmpty {
                return "Overnight: off"
            }
            return "Overnight: asks at \(s.nightAskAt)"
        default:
            return "Overnight: asks at 23:00"
        }
    }

    func formatUntilClock(_ iso: String) -> String? {
        guard !iso.isEmpty else { return nil }
        let f = ISO8601DateFormatter()
        guard let d = f.date(from: iso) else { return nil }
        let cal = Calendar.current
        let h = cal.component(.hour, from: d)
        let m = cal.component(.minute, from: d)
        if m == 0 {
            return String(format: "%d:00", h)
        }
        return String(format: "%d:%02d", h, m)
    }

    func latestPrimeTime(_ pl: [String: Any]) -> String {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let today = Calendar.current.startOfDay(for: Date())
        var best: Date?
        for (_, v) in pl {
            guard let ent = v as? [String: Any], ent["ok"] as? Bool == true,
                  let ts = ent["ts"] as? String else { continue }
            var d = f.date(from: ts)
            if d == nil {
                f.formatOptions = [.withInternetDateTime]
                d = f.date(from: ts)
                f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
            }
            guard let dt = d, dt >= today else { continue }
            if best == nil || dt > best! { best = dt }
        }
        guard let b = best else { return "" }
        let cal = Calendar.current
        return String(format: "%d:%02d", cal.component(.hour, from: b), cal.component(.minute, from: b))
    }

    func windowStatusLine(_ s: Status) -> String {
        if s.primeAt.isEmpty {
            return "Window: not scheduled"
        }
        let next = formatUntilClock(s.primeNextAt) ?? ""
        if !s.primeLastOK.isEmpty {
            if next.isEmpty {
                return "Window: primed \(s.primeLastOK)"
            }
            return "Window: primed \(s.primeLastOK) · next \(next)"
        }
        if next.isEmpty {
            return "Window: not scheduled"
        }
        return "Window: next \(next)"
    }

    func registerNightCategory() {
        let yes = UNNotificationAction(identifier: "keepgoing.night.yes", title: "Keep running", options: [])
        let no = UNNotificationAction(identifier: "keepgoing.night.no", title: "Let it sleep", options: [])
        let cat = UNNotificationCategory(identifier: "keepgoing.night", actions: [yes, no], intentIdentifiers: [], options: [])
        UNUserNotificationCenter.current().setNotificationCategories([cat])
    }

    func handleNightRequest(id: Int, mode: String) {
        guard mode == "ask", id > 0, id != lastNightRequestID else { return }
        lastNightRequestID = id
        ensureNightNotification { granted in
            if granted {
                self.postNightNotification()
            } else {
                self.showNightMenuFallback()
            }
        }
    }

    func ensureNightNotification(completion: @escaping (Bool) -> Void) {
        UNUserNotificationCenter.current().getNotificationSettings { settings in
            switch settings.authorizationStatus {
            case .notDetermined:
                UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { ok, _ in
                    DispatchQueue.main.async { completion(ok) }
                }
            case .authorized, .provisional, .ephemeral:
                DispatchQueue.main.async { completion(true) }
            default:
                DispatchQueue.main.async { completion(false) }
            }
        }
    }

    func postNightNotification() {
        let content = UNMutableNotificationContent()
        content.title = "KeepGoing"
        content.body = "Keep your Mac awake overnight? Agents are still running. No answer in 10 minutes means sleep."
        content.categoryIdentifier = "keepgoing.night"
        let req = UNNotificationRequest(identifier: "keepgoing-night", content: content, trigger: nil)
        UNUserNotificationCenter.current().add(req)
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse, withCompletionHandler completionHandler: @escaping () -> Void) {
        switch response.actionIdentifier {
        case "keepgoing.night.yes":
            postNightAnswer("yes")
        case "keepgoing.night.no":
            postNightAnswer("no")
        default:
            break
        }
        completionHandler()
    }

    func showNightMenuFallback() {
        nightMenuFallback = true
        item.button?.image = symbol("moon.zzz")
        renderedIcon = "moon.zzz"
        nightItem.isHidden = false
        setTitle(nightItem, to: "Overnight: asking — choose below", store: &renderedNight)
        nightYesItem.isHidden = false
        nightNoItem.isHidden = false
    }

    func clearNightFallback() {
        guard nightMenuFallback else { return }
        nightMenuFallback = false
        nightYesItem.isHidden = true
        nightNoItem.isHidden = true
        renderedIcon = ""
        render()
    }

    func postNightAnswer(_ answer: String) {
        var req = URLRequest(url: URL(string: "http://127.0.0.1:7777/_night")!)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try? JSONSerialization.data(withJSONObject: ["answer": answer])
        URLSession.shared.dataTask(with: req).resume()
        clearNightFallback()
    }

    @objc func nightAnswerYes() { postNightAnswer("yes") }
    @objc func nightAnswerNo() { postNightAnswer("no") }

    @objc func toggleOvernight() {
        let on = status.nightAnswer != "yes"
        run([cli, "night", on ? "yes" : "no"])
        restartDaemon()
    }

    func postThermalNotification() {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { granted, _ in
            guard granted else { return }
            let content = UNMutableNotificationContent()
            content.title = "KeepGoing"
            content.body = "Mac is running hot with the lid closed. Open it or move it off soft surfaces."
            let req = UNNotificationRequest(identifier: "keepgoing-thermal", content: content, trigger: nil)
            UNUserNotificationCenter.current().add(req)
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

    func appLogPath() -> String {
        NSHomeDirectory() + "/Library/Logs/keepgoing/app.log"
    }

    func appLog(_ msg: String) {
        let path = appLogPath()
        try? FileManager.default.createDirectory(atPath: (path as NSString).deletingLastPathComponent, withIntermediateDirectories: true)
        let line = "\(ISO8601DateFormatter().string(from: Date())) \(msg)\n"
        if FileManager.default.fileExists(atPath: path), let h = try? FileHandle(forWritingTo: URL(fileURLWithPath: path)) {
            h.seekToEndOfFile()
            h.write(line.data(using: .utf8)!)
            try? h.close()
        } else {
            FileManager.default.createFile(atPath: path, contents: line.data(using: .utf8), attributes: [.posixPermissions: 0o644])
        }
    }

    func applicationWillTerminate(_ notification: Notification) {
        appLog("exiting reason=\(exitReason)")
    }

    @objc func quitStopAutostart() {
        exitReason = "user quit (stop autostart)"
        run(["/bin/launchctl", "bootout", "gui/\(getuid())/com.elijah.keepgoing.app"])
        NSApp.terminate(nil)
    }

    func ensureDaemon() {
        let (code, _) = run(["/bin/launchctl", "print", "gui/\(getuid())/\(daemonLabel)"])
        if code != 0 {
            appLog("daemon missing — running keepgoing install")
            run([cli, "install"])
        }
    }

    @objc func startDaemon() {
        let (code, _) = run(["/bin/launchctl", "print", "gui/\(getuid())/\(daemonLabel)"])
        if code != 0 {
            run([cli, "install"])
        } else {
            run(["/bin/launchctl", "kickstart", "-k", "gui/\(getuid())/\(daemonLabel)"])
        }
        failCount = 0
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) { self.refresh() }
    }

    @objc func restartDaemon() {
        run(["/bin/launchctl", "kickstart", "-k", "gui/\(getuid())/\(daemonLabel)"])
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

    @objc func toggleIdleSleep() {
        setConfigKey("idle_sleep_after", value: status.idleSleepAfter == 0 ? 30 : 0)
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
            /usr/bin/pmset -a lowpowermode 1
            /usr/bin/pmset -a lowpowermode 0
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
        alert.informativeText = hotspotGuidance
        alert.addButton(withTitle: "Save")
        alert.addButton(withTitle: "Open Wi-Fi settings")
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
        let choice = alert.runModal()
        if choice == .alertSecondButtonReturn {
            if let url = URL(string: "x-apple.systempreferences:com.apple.wifi-settings-extension") {
                NSWorkspace.shared.open(url)
            }
            return
        }
        guard choice == .alertFirstButtonReturn, !ssid.stringValue.isEmpty, !pw.stringValue.isEmpty else { return }
        let (code, out) = run([cli, "hotspot", "set", ssid.stringValue], input: pw.stringValue)
        if code != 0 {
            showError("Couldn't save hotspot", detail: out)
        } else {
            wifi.ensureLocation()
        }
        restartDaemon()
    }

    @objc func openLog() {
        NSWorkspace.shared.open(URL(fileURLWithPath: NSHomeDirectory() + "/Library/Logs/keepgoing/daemon.log"))
    }

    @objc func openThermalLog() {
        let path = NSHomeDirectory() + "/Library/Logs/keepgoing/thermal.csv"
        let url = URL(fileURLWithPath: path)
        if !FileManager.default.fileExists(atPath: path) {
            let hdr = "ts_iso,lid_closed,thermal_state,cpu_c,low_power,cool_pids,agents,working,on_battery\n"
            FileManager.default.createFile(atPath: path, contents: Data(hdr.utf8))
        }
        NSWorkspace.shared.open(url)
    }

    func appLoginPlistPath() -> String {
        NSHomeDirectory() + "/Library/LaunchAgents/com.elijah.keepgoing.app.plist"
    }

    func appLoginEnabled() -> Bool {
        FileManager.default.fileExists(atPath: appLoginPlistPath())
    }

    @objc func toggleLogin() {
        if appLoginEnabled() {
            run([cli, "app", "login", "off"])
        } else {
            run([cli, "app", "login", "on"])
        }
        loginItem.state = appLoginEnabled() ? .on : .off
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
