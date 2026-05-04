import Foundation

enum OrderMapper {
    static func decodeList(from data: Data, provider: Provider) throws -> [OrderSummary] {
        let root = try JSONSerialization.jsonObject(with: data)
        let object = root as? [String: Any]
        let orders = (object?["orders"] as? [[String: Any]]) ?? (root as? [[String: Any]]) ?? []
        return orders.compactMap { row in
            let id = string(row["id"] ?? row["orderId"])
            guard !id.isEmpty else { return nil }
            let rowProvider = Provider(rawValue: string(row["provider"])) ?? provider
            return OrderSummary(
                id: id,
                provider: rowProvider,
                status: string(row["status"] ?? row["orderStatus"]),
                createdAt: optionalString(row["createdAt"] ?? row["created_at"] ?? row["placedAt"]),
                total: money(row["total"]) ?? moneyFromPLN(row["totalPLN"])
            )
        }
    }

    static func prettyJSONString(from data: Data) -> String {
        guard let object = try? JSONSerialization.jsonObject(with: data),
              let pretty = try? JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys]),
              let text = String(data: pretty, encoding: .utf8) else {
            return String(data: data, encoding: .utf8)?.redactedForDisplay ?? ""
        }
        return text.redactedForDisplay
    }

    private static func money(_ value: Any?) -> Money? {
        guard let object = value as? [String: Any], let amount = decimal(object["amount"]) else { return nil }
        return Money(amount: amount, currency: optionalString(object["currency"]) ?? "PLN")
    }

    private static func moneyFromPLN(_ value: Any?) -> Money? {
        guard let amount = decimal(value) else { return nil }
        return Money(amount: amount, currency: "PLN")
    }

    private static func decimal(_ value: Any?) -> Decimal? {
        switch value {
        case let value as Double: Decimal(value)
        case let value as Int: Decimal(value)
        case let value as String: Decimal(string: value)
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
