import Foundation

enum ProductSearchMapper {
    static func decodeProducts(from data: Data, provider: Provider) throws -> [ProductCard] {
        let root = try JSONSerialization.jsonObject(with: data)
        switch provider {
        case .frisco:
            return friscoProducts(root)
        case .delio:
            return delioProducts(root)
        }
    }

    private static func friscoProducts(_ root: Any) -> [ProductCard] {
        guard let object = root as? [String: Any] else { return [] }
        let rows = object["products"] as? [[String: Any]] ?? []
        return rows.compactMap { row in
            let product = (row["product"] as? [String: Any]) ?? row
            let id = string(product["productId"] ?? product["id"])
            guard !id.isEmpty else { return nil }
            let priceObject = product["price"] as? [String: Any]
            let regular = decimal(priceObject?["price"]) ?? 0
            let promo = decimal(priceObject?["priceAfterPromotion"])
            return ProductCard(
                id: id,
                provider: .frisco,
                sku: id,
                name: localized(product["name"]),
                brand: optionalString(product["brand"]),
                imageURL: URL(string: string(product["imageUrl"])),
                price: Money(amount: regular, currency: "PLN"),
                promoPrice: promo.map { Money(amount: $0, currency: "PLN") },
                promoCondition: friscoPromoCondition(product),
                deposit: decimal(product["depositFee"]).map { Money(amount: $0, currency: "PLN") },
                measureDisplay: measureDisplay(value: product["grammage"], unit: optionalString(product["unitOfMeasure"])),
                available: bool(product["isAvailable"]) ?? bool(product["isStocked"]) ?? true,
                availableQuantity: int(product["stock"] ?? product["unrestrictedStock"])
            )
        }
    }

    private static func delioProducts(_ root: Any) -> [ProductCard] {
        guard let object = root as? [String: Any],
              let data = object["data"] as? [String: Any],
              let search = data["productSearch"] as? [String: Any] else { return [] }
        let rows = search["results"] as? [[String: Any]] ?? []
        return rows.compactMap { product in
            let id = string(product["key"] ?? product["sku"] ?? product["id"])
            guard !id.isEmpty else { return nil }
            let attributes = product["attributes"] as? [String: Any]
            let priceValue = ((product["price"] as? [String: Any])?["value"] as? [String: Any])
            let minor = decimal(priceValue?["centAmount"]) ?? 0
            let fraction = decimal(priceValue?["fractionDigits"]) ?? 2
            let amount = minor / pow10(fraction)
            let depositMinor = (((product["depositFee"] as? [String: Any])?["value"] as? [String: Any])?["centAmount"])
            let deposit = decimal(depositMinor).map { Money(amount: $0 / 100, currency: "PLN") }
            return ProductCard(
                id: id,
                provider: .delio,
                sku: string(product["sku"] ?? product["key"]),
                name: string(product["name"]),
                brand: nil,
                imageURL: URL(string: (product["imagesUrls"] as? [String])?.first ?? ""),
                price: Money(amount: amount, currency: "PLN"),
                promoPrice: nil,
                promoCondition: nil,
                deposit: deposit,
                measureDisplay: measureDisplay(value: attributes?["net_contain"], unit: optionalString(attributes?["contain_unit"])),
                available: (int(product["availableQuantity"]) ?? 0) > 0,
                availableQuantity: int(product["availableQuantity"])
            )
        }
    }

    private static func friscoPromoCondition(_ product: [String: Any]) -> String? {
        guard let promos = product["promotions"] as? [[String: Any]], let first = promos.first else { return nil }
        let content = first["contentData"] as? [String: Any]
        return optionalString(content?["info"] ?? content?["text"] ?? first["campaignName"])
    }

    private static func localized(_ value: Any?) -> String {
        if let dict = value as? [String: Any] {
            return string(dict["pl"] ?? dict["en"])
        }
        return string(value)
    }

    private static func measureDisplay(value: Any?, unit: String?) -> String? {
        guard let amount = optionalString(value), !amount.isEmpty else { return nil }
        return [amount, unit].compactMap { $0 }.joined(separator: " ")
    }

    private static func pow10(_ exponent: Decimal) -> Decimal {
        let intExponent = NSDecimalNumber(decimal: exponent).intValue
        return Decimal(pow(10.0, Double(intExponent)))
    }

    private static func decimal(_ value: Any?) -> Decimal? {
        switch value {
        case let value as Decimal: value
        case let value as Double: Decimal(value)
        case let value as Int: Decimal(value)
        case let value as Int64: Decimal(value)
        case let value as String: Decimal(string: value.replacingOccurrences(of: ",", with: "."))
        default: nil
        }
    }

    private static func int(_ value: Any?) -> Int? {
        switch value {
        case let value as Int: value
        case let value as Int64: Int(value)
        case let value as Double: Int(value)
        case let value as String: Int(value)
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

    private static func string(_ value: Any?) -> String { optionalString(value) ?? "" }

    private static func optionalString(_ value: Any?) -> String? {
        guard let value else { return nil }
        let text = String(describing: value).trimmingCharacters(in: .whitespacesAndNewlines)
        return text.isEmpty || text == "<null>" ? nil : text
    }
}
