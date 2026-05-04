import Foundation

enum Provider: String, Codable, CaseIterable, Identifiable {
    case frisco
    case delio

    var id: String { rawValue }
    var displayName: String { rawValue.capitalized }
}

struct Money: Codable, Hashable {
    var amount: Decimal
    var currency: String

    var display: String {
        let number = NSDecimalNumber(decimal: amount)
        return "\(number.stringValue) \(currency)"
    }
}

struct ProductCard: Codable, Identifiable, Hashable {
    var id: String
    var provider: Provider
    var sku: String?
    var name: String
    var brand: String?
    var imageURL: URL?
    var price: Money
    var promoPrice: Money?
    var promoCondition: String?
    var deposit: Money?
    var measureDisplay: String?
    var available: Bool
    var availableQuantity: Int?

    static let seed: [ProductCard] = [
        ProductCard(
            id: "154030",
            provider: .frisco,
            sku: "154030",
            name: "Coca-Cola Zero 330 ml",
            brand: "COCA-COLA",
            imageURL: URL(string: "https://res.cloudinary.com/dj484tw6k/image/upload/v1764805580/154030.png"),
            price: Money(amount: 4.59, currency: "PLN"),
            promoPrice: Money(amount: 3.49, currency: "PLN"),
            promoCondition: "przy zakupie 6 szt.",
            deposit: Money(amount: 0.50, currency: "PLN"),
            measureDisplay: "330 ml",
            available: true,
            availableQuantity: 3761
        )
    ]
}

struct CartItem: Codable, Identifiable, Hashable {
    var id: String { "\(product.provider.rawValue):\(product.id)" }
    var product: ProductCard
    var quantity: Int
    var unitPrice: Money
    var lineTotal: Money
    var depositTotal: Money?
    var warnings: [String]
}

struct CartSummary: Codable, Hashable {
    var provider: Provider
    var cartID: String?
    var items: [CartItem]
    var grandTotal: Money?
    var warnings: [String]

    static let seed = CartSummary(provider: .frisco, cartID: "cart-1", items: [], grandTotal: nil, warnings: [])
}

struct ReservationWindow: Codable, Hashable {
    var startsAt: String?
    var endsAt: String?
    var deliveryMethod: String?
}

struct ReservationSlot: Identifiable, Codable, Hashable {
    var id: String { "\(date)|\(from)|\(to)|\(deliveryMethod ?? "")|\(warehouse ?? "")" }
    var date: String
    var from: String
    var to: String
    var deliveryMethod: String?
    var warehouse: String?
    var available: Bool
    var price: Money?

    var displayTime: String { "\(from)–\(to)" }
}

struct PaymentSelection: Codable, Hashable {
    var method: String?
    var channel: String?
    var status: String?
}

struct CheckoutIssue: Codable, Hashable, Identifiable {
    var id: String { code + message }
    var code: String
    var message: String
}

struct CheckoutPreview: Codable, Hashable {
    var provider: Provider
    var userID: String
    var cartID: String?
    var itemCount: Int
    var total: Money?
    var reservation: ReservationWindow?
    var payment: PaymentSelection?
    var readyToFinalize: Bool
    var issues: [CheckoutIssue]
}

struct SessionStatus: Identifiable, Hashable {
    var id: Provider { provider }
    var provider: Provider
    var saved: Bool
    var authPresent: Bool
    var baseURL: String
    var userID: String
    var tokenSaved: Bool
    var refreshTokenSaved: Bool
    var cookieSaved: Bool
    var sessionFile: String?
}

struct CatalogProduct: Identifiable, Hashable {
    var id: Int64
    var provider: Provider
    var externalID: String
    var name: String
    var brand: String?
    var price: Money?
    var measureText: String?
    var available: Bool?
    var lastSeenAt: String
}

struct PriceSnapshot: Identifiable, Hashable {
    var id: Int64
    var seenAt: String
    var source: String
    var price: Money?
    var promoPrice: Money?
    var available: Bool?
}

struct OrderSummary: Codable, Identifiable, Hashable {
    var id: String
    var provider: Provider
    var status: String
    var createdAt: String?
    var total: Money?

    static let seed: [OrderSummary] = []
}

struct ShoppingAction: Identifiable, Codable, Hashable {
    enum Kind: String, Codable {
        case searchProducts
        case addToCart
        case showCart
        case checkoutPreview
        case checkoutFinalize
    }

    var id = UUID()
    var kind: Kind
    var provider: Provider?
    var title: String
    var arguments: [String: String]
    var requiresExplicitApproval: Bool = true
}

struct PiResponse: Hashable {
    var text: String
    var actions: [ShoppingAction]
    var productCards: [ProductCard] = []
    var usedRealPi: Bool = false
}

struct ChatMessage: Identifiable, Hashable {
    enum Role: String { case user, assistant, system }
    var id = UUID()
    var role: Role
    var text: String
    var productIDs: [ProductCard.ID] = []

    static let seed = [
        ChatMessage(role: .assistant, text: "Napisz, co chcesz kupić. Mogę porównać Frisco i Delio, dodać do koszyka i przygotować checkout preview.")
    ]
}
