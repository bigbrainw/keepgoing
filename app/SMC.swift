// SMC CPU temperature via IOKit (no root). Struct layout matches SMCKit/smol.
import IOKit

private enum SMCCommand {
    static let readKey: UInt8 = 5
    static let getKeyInfo: UInt8 = 9
}

private struct SMCVersion {
    var major: UInt8 = 0
    var minor: UInt8 = 0
    var build: UInt8 = 0
    var reserved: UInt8 = 0
    var release: UInt16 = 0
}

private struct SMCPLimitData {
    var version: UInt16 = 0
    var length: UInt16 = 0
    var cpuPLimit: UInt32 = 0
    var gpuPLimit: UInt32 = 0
    var memPLimit: UInt32 = 0
}

private struct SMCKeyInfoData {
    var dataSize: UInt32 = 0
    var dataType: UInt32 = 0
    var dataAttributes: UInt8 = 0
}

private struct SMCKeyData {
    var key: UInt32 = 0
    var vers = SMCVersion()
    var pLimitData = SMCPLimitData()
    var keyInfo = SMCKeyInfoData()
    var padding: UInt16 = 0
    var result: UInt8 = 0
    var status: UInt8 = 0
    var data8: UInt8 = 0
    var data32: UInt32 = 0
    var bytes: (UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8,
                UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8,
                UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8,
                UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8) =
        (0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
         0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
}

private func fourCharCode(_ key: String) -> UInt32 {
    var result: UInt32 = 0
    for (i, c) in key.utf8.enumerated() where i < 4 {
        result = result << 8 | UInt32(c)
    }
    let pad = 4 - min(key.utf8.count, 4)
    for _ in 0..<pad { result = result << 8 | 0x20 }
    return result
}

private func decodeTemp(type: UInt32, bytes: (UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8,
    UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8,
    UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8,
    UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8, UInt8), size: UInt32) -> Double? {
    let sp78 = fourCharCode("sp78")
    let sp87 = fourCharCode("sp87")
    let fpe2 = fourCharCode("fpe2")
    let flt = fourCharCode("flt ")
    if type == sp78 || type == sp87 {
        let t = Double(Int8(bitPattern: bytes.0)) + Double(bytes.1) / 256.0
        return t
    }
    if type == fpe2 {
        let raw = UInt16(bytes.0) << 8 | UInt16(bytes.1)
        return Double(raw) / 4.0
    }
    if type == flt && size >= 4 {
        let le = UInt32(bytes.0) | UInt32(bytes.1) << 8 | UInt32(bytes.2) << 16 | UInt32(bytes.3) << 24
        let v = Double(Float(bitPattern: le))
        if v > 10 && v < 150 { return v }
        let be = UInt32(bytes.0) << 24 | UInt32(bytes.1) << 16 | UInt32(bytes.2) << 8 | UInt32(bytes.3)
        let v2 = Double(Float(bitPattern: be))
        if v2 > 10 && v2 < 150 { return v2 }
    }
    if bytes.0 == 0 && bytes.1 == 0 { return nil }
    let t = Double(Int8(bitPattern: bytes.0)) + Double(bytes.1) / 256.0
    return t
}

final class SMCReader {
    private var conn: io_connect_t = 0

    init?() {
        for name in ["AppleSMC", "AppleSMCKeysEndpoint"] {
            let svc = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching(name))
            if svc == 0 { continue }
            defer { IOObjectRelease(svc) }
            if IOServiceOpen(svc, mach_task_self_, 0, &conn) == KERN_SUCCESS {
                return
            }
        }
        return nil
    }

    deinit {
        if conn != 0 { IOServiceClose(conn) }
    }

    private func call(_ input: inout SMCKeyData, _ output: inout SMCKeyData) -> Bool {
        var outSize = MemoryLayout<SMCKeyData>.size
        return IOConnectCallStructMethod(
            conn, 2,
            &input, MemoryLayout<SMCKeyData>.size,
            &output, &outSize
        ) == KERN_SUCCESS
    }

    private func readKey(_ key: String) -> Double? {
        var input = SMCKeyData()
        var output = SMCKeyData()
        input.key = fourCharCode(key)
        input.data8 = SMCCommand.getKeyInfo
        guard call(&input, &output), output.keyInfo.dataSize > 0 else { return nil }
        let dataSize = output.keyInfo.dataSize
        let dataType = output.keyInfo.dataType

        input = SMCKeyData()
        output = SMCKeyData()
        input.key = fourCharCode(key)
        input.data8 = SMCCommand.readKey
        input.keyInfo.dataSize = dataSize
        guard call(&input, &output) else { return nil }
        guard let t = decodeTemp(type: dataType, bytes: output.bytes, size: dataSize) else { return nil }
        if t < 10 || t > 120 { return nil }
        return t
    }

    func cpuCelsius() -> Double? {
        #if arch(arm64)
        let keys = ["Tp09", "Tp0T", "Tp01", "Tp05", "Tp0D", "Tp0H", "Tp0L", "Tp0P", "Tp0X", "Tp0b"]
        var vals: [Double] = []
        for k in keys {
            if let t = readKey(k) { vals.append(t) }
        }
        guard !vals.isEmpty else { return nil }
        return vals.reduce(0, +) / Double(vals.count)
        #else
        return readKey("TC0P")
        #endif
    }
}
