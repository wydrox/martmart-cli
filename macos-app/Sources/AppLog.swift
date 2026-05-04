import Foundation

enum AppLog {
    static let url = URL(fileURLWithPath: "/tmp/martmart-shopping-chat-app.log")

    static func write(_ message: String) {
        let line = "[\(ISO8601DateFormatter().string(from: Date()))] \(message.redactedForDisplay)\n"
        guard let data = line.data(using: .utf8) else { return }
        if FileManager.default.fileExists(atPath: url.path) {
            if let handle = try? FileHandle(forWritingTo: url) {
                defer { try? handle.close() }
                _ = try? handle.seekToEnd()
                try? handle.write(contentsOf: data)
            }
        } else {
            try? data.write(to: url, options: .atomic)
        }
    }
}
