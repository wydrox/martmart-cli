import Foundation

enum CheckoutMapper {
    static func decodePreview(from data: Data) throws -> CheckoutPreview {
        let root = try JSONSerialization.jsonObject(with: data)
        guard let rootObject = root as? [String: Any] else {
            throw MartMartError.commandFailed("Nieprawidłowy checkout preview JSON")
        }
        let object = (rootObject["preview"] as? [String: Any]) ?? rootObject
        let provider = Provider(rawValue: string(object["provider"])) ?? .frisco
        return CheckoutPreview(
            provider: provider,
            userID: string(object["user_id"] ?? object["userID"]),
            cartID: optionalString(object["cart_id"] ?? object["cartID"]),
            itemCount: int(object["item_count"] ?? object["itemCount"]),
            total: money(object["total"]),
            reservation: reservation(object["reservation"]),
            payment: payment(object["payment"]),
            readyToFinalize: bool(object["ready_to_finalize"] ?? object["readyToFinalize"]) ?? false,
            issues: issues(object["issues"])
        )
    }

    static func decodeFinalizeResult(from data: Data) throws -> CheckoutFinalizeResultViewModel {
        let root = try JSONSerialization.jsonObject(with: data)
        guard let rootObject = root as? [String: Any] else {
            throw MartMartError.commandFailed("Nieprawidłowy checkout finalize JSON")
        }
        let object = (rootObject["result"] as? [String: Any]) ?? rootObject
        let action = action(object["action"])
        return CheckoutFinalizeResultViewModel(
            status: string(object["status"]),
            orderID: optionalString(object["order_id"] ?? object["orderID"]),
            action: action
        )
    }

    private static func reservation(_ value: Any?) -> ReservationWindow? {
        guard let object = value as? [String: Any] else { return nil }
        return ReservationWindow(
            startsAt: optionalString(object["starts_at"] ?? object["startsAt"]),
            endsAt: optionalString(object["ends_at"] ?? object["endsAt"]),
            deliveryMethod: optionalString(object["delivery_method"] ?? object["deliveryMethod"])
        )
    }

    private static func payment(_ value: Any?) -> PaymentSelection? {
        guard let object = value as? [String: Any] else { return nil }
        return PaymentSelection(
            method: optionalString(object["method"]),
            channel: optionalString(object["channel"]),
            status: optionalString(object["status"])
        )
    }

    private static func issues(_ value: Any?) -> [CheckoutIssue] {
        guard let rows = value as? [[String: Any]] else { return [] }
        return rows.map { row in
            CheckoutIssue(code: string(row["code"]), message: string(row["message"]))
        }
    }

    private static func action(_ value: Any?) -> PaymentActionViewModel? {
        guard let object = value as? [String: Any] else { return nil }
        return PaymentActionViewModel(
            kind: string(object["kind"]),
            url: optionalString(object["url"]),
            method: optionalString(object["method"]),
            message: optionalString(object["message"])
        )
    }

    private static func money(_ value: Any?) -> Money? {
        guard let object = value as? [String: Any], let amount = decimal(object["amount"]) else { return nil }
        return Money(amount: amount, currency: optionalString(object["currency"]) ?? "PLN")
    }

    private static func decimal(_ value: Any?) -> Decimal? {
        switch value {
        case let value as Double: Decimal(value)
        case let value as Int: Decimal(value)
        case let value as String: Decimal(string: value)
        default: nil
        }
    }

    private static func int(_ value: Any?) -> Int {
        switch value {
        case let value as Int: value
        case let value as Double: Int(value)
        case let value as String: Int(value) ?? 0
        default: 0
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

struct CheckoutFinalizeResultViewModel: Hashable {
    var status: String
    var orderID: String?
    var action: PaymentActionViewModel?
}

struct PaymentActionViewModel: Hashable {
    var kind: String
    var url: String?
    var method: String?
    var message: String?
}
