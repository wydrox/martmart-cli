import Foundation

enum ReservationSlotMapper {
    static func decodeSlots(from data: Data) throws -> [ReservationSlot] {
        let root = try JSONSerialization.jsonObject(with: data)
        guard let object = root as? [String: Any] else { return [] }
        let days = object["days"] as? [[String: Any]] ?? []
        return days.flatMap { day -> [ReservationSlot] in
            let date = string(day["date"])
            let slots = day["slots"] as? [[String: Any]] ?? []
            return slots.compactMap { slot in
                makeSlot(slot, fallbackDate: date)
            }
        }
    }

    private static func makeSlot(_ raw: [String: Any], fallbackDate: String) -> ReservationSlot? {
        let startsAt = string(raw["startsAt"] ?? raw["start"] ?? raw["fromDate"])
        let endsAt = string(raw["endsAt"] ?? raw["end"] ?? raw["toDate"])
        let date = datePart(startsAt) ?? fallbackDate
        let from = timePart(startsAt) ?? string(raw["from"])
        let to = timePart(endsAt) ?? string(raw["to"])
        guard !date.isEmpty, !from.isEmpty, !to.isEmpty else { return nil }
        return ReservationSlot(
            date: date,
            from: from,
            to: to,
            deliveryMethod: optionalString(raw["deliveryMethod"] ?? raw["method"]),
            warehouse: optionalString(raw["warehouse"]),
            available: bool(raw["available"]) ?? true,
            price: money(raw["price"])
        )
    }

    private static func money(_ value: Any?) -> Money? {
        guard let object = value as? [String: Any] else { return nil }
        if let amount = decimal(object["amount"]), let currency = optionalString(object["currency"]) {
            return Money(amount: amount, currency: currency)
        }
        return nil
    }

    private static func decimal(_ value: Any?) -> Decimal? {
        switch value {
        case let value as Double: Decimal(value)
        case let value as Int: Decimal(value)
        case let value as String: Decimal(string: value)
        default: nil
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

    private static func datePart(_ iso: String) -> String? {
        guard iso.count >= 10 else { return nil }
        return String(iso.prefix(10))
    }

    private static func timePart(_ iso: String) -> String? {
        guard let t = iso.firstIndex(of: "T") else { return nil }
        let afterT = iso[iso.index(after: t)...]
        guard afterT.count >= 5 else { return String(afterT) }
        return String(afterT.prefix(5))
    }
}
