import Foundation

struct MartMartClient: Sendable {
    var executableURL: URL

    init(executableURL: URL? = nil) {
        if let executableURL {
            self.executableURL = executableURL
        } else {
            let fm = FileManager.default
            let cwd = URL(fileURLWithPath: fm.currentDirectoryPath)
            let bundled = Bundle.main.resourceURL?.appending(path: "martmart")
            let candidates = [
                bundled,
                cwd.appending(path: "martmart"),
                cwd.deletingLastPathComponent().appending(path: "martmart"),
                URL(fileURLWithPath: "/Users/rafalw/dev/martmart-cli/martmart")
            ].compactMap { $0 }
            if let found = candidates.first(where: { fm.isExecutableFile(atPath: $0.path) }) {
                self.executableURL = found
            } else {
                self.executableURL = URL(fileURLWithPath: "/usr/bin/env")
            }
        }
    }

    func runJSON(arguments: [String]) async throws -> Data {
        let box = ProcessBox()
        return try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { continuation in
                let process = Process()
                box.process = process
                process.executableURL = executableURL
                if executableURL.path == "/usr/bin/env" {
                    process.arguments = ["martmart", "--format", "json"] + arguments
                } else {
                    process.arguments = ["--format", "json"] + arguments
                }
                AppLog.write("martmart start: \(([process.executableURL?.path ?? "martmart"] + (process.arguments ?? [])).joined(separator: " "))")

                let output = Pipe()
                let error = Pipe()
                process.standardOutput = output
                process.standardError = error

                process.terminationHandler = { process in
                    let data = output.fileHandleForReading.readDataToEndOfFile()
                    let errData = error.fileHandleForReading.readDataToEndOfFile()
                    AppLog.write("martmart exit status=\(process.terminationStatus) stdout=\(data.count)B stderr=\(errData.count)B")
                    if process.terminationStatus == 0 {
                        continuation.resume(returning: data)
                    } else if box.wasCancelled {
                        continuation.resume(throwing: CancellationError())
                    } else {
                        let message = String(data: errData, encoding: .utf8) ?? "martmart failed"
                        continuation.resume(throwing: MartMartError.commandFailed(message.redactedForDisplay))
                    }
                    box.process = nil
                }

                do {
                    try process.run()
                } catch {
                    AppLog.write("martmart launch failed: \(error.localizedDescription)")
                    continuation.resume(throwing: error)
                    box.process = nil
                }
            }
        } onCancel: {
            box.cancel()
        }
    }
}

final class ProcessBox: @unchecked Sendable {
    private let lock = NSLock()
    private var _process: Process?
    private var _wasCancelled = false

    var process: Process? {
        get {
            lock.withLock { _process }
        }
        set {
            lock.withLock { _process = newValue }
        }
    }

    var wasCancelled: Bool {
        lock.withLock { _wasCancelled }
    }

    func cancel() {
        let process: Process? = lock.withLock {
            _wasCancelled = true
            return _process
        }
        process?.terminate()
    }
}

enum MartMartError: LocalizedError {
    case commandFailed(String)

    var errorDescription: String? {
        switch self {
        case .commandFailed(let message): message
        }
    }
}

extension String {
    var redactedForDisplay: String {
        replacingOccurrences(of: #"(?i)(Bearer\s+)[A-Za-z0-9._-]+"#, with: "$1***", options: .regularExpression)
            .replacingOccurrences(of: #"(?i)(idToken|refreshToken|authToken|Authorization|Cookie)(=|: )[^;\s]+"#, with: "$1$2***", options: .regularExpression)
    }
}
