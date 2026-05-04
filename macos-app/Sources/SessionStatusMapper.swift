import Foundation

enum SessionStatusMapper {
    static func decodeList(from data: Data) throws -> [SessionStatus] {
        let root = try JSONSerialization.jsonObject(with: data)
        let rows = root as? [[String: Any]] ?? []
        return rows.compactMap { row in
            guard let provider = Provider(rawValue: string(row["provider"])) else { return nil }
            return SessionStatus(
                provider: provider,
                saved: bool(row["saved"]) ?? false,
                authPresent: bool(row["auth_present"] ?? row["authenticated"]) ?? false,
                baseURL: string(row["base_url"]),
                userID: string(row["user_id"]),
                tokenSaved: bool(row["token_saved"]) ?? false,
                refreshTokenSaved: bool(row["refresh_token_saved"]) ?? false,
                cookieSaved: bool(row["cookie_saved"]) ?? false,
                sessionFile: optionalString(row["session_file"])
            )
        }
    }

    private static func bool(_ value: Any?) -> Bool? {
        switch value {
        case let value as Bool: value
        case let value as String: ["true", "1", "yes"].contains(value.lowercased())
        default: nil
        }
    }

    private static func string(_ value: Any?) -> String {
        optionalString(value) ?? ""
    }

    private static func optionalString(_ value: Any?) -> String? {
        guard let value else { return nil }
        let text = String(describing: value).trimmingCharacters(in: .whitespacesAndNewlines)
        return text.isEmpty || text == "<null>" ? nil : text
    }
}
