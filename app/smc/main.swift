// keepgoing-smc — read CPU °C and thermal state; one JSON line on stdout.
import Foundation

private func thermalName() -> String {
    switch ProcessInfo.processInfo.thermalState {
    case .nominal: return "nominal"
    case .fair: return "fair"
    case .serious: return "serious"
    case .critical: return "critical"
    @unknown default: return "nominal"
    }
}

struct Out: Encodable {
    let cpu_c: Double?
    let keys: [String: Double]
    let thermal: String
}

let reader = SMCReader()
let keys = reader?.cpuKeys() ?? [:]
var cpuC: Double?
if !keys.isEmpty {
    cpuC = keys.values.reduce(0, +) / Double(keys.count)
}
let payload = Out(cpu_c: cpuC, keys: keys, thermal: thermalName())
if let data = try? JSONEncoder().encode(payload),
   let line = String(data: data, encoding: .utf8) {
    print(line)
}
