import Foundation
import Observation

@MainActor
@Observable
final class AppState {
    enum Tab: String, CaseIterable, Identifiable {
        case chat = "Chat"
        case cart = "Cart"
        case checkout = "Checkout"
        case orders = "Orders"
        case history = "History"
        case session = "Session"

        var id: String { rawValue }
    }

    var selectedTab: Tab = .chat
    var selectedProvider: Provider = .frisco
    var messages: [ChatMessage] = ChatMessage.seed
    var productCards: [ProductCard] = ProductCard.seed
    var cart: CartSummary = .seed
    var checkoutPreview: CheckoutPreview?
    var reservationSlots: [ReservationSlot] = []
    var selectedReservationSlot: ReservationSlot?
    var checkoutResult: CheckoutFinalizeResultViewModel?
    var orders: [OrderSummary] = OrderSummary.seed
    var selectedOrder: OrderSummary?
    var catalogProducts: [CatalogProduct] = []
    var selectedCatalogProduct: CatalogProduct?
    var priceSnapshots: [PriceSnapshot] = []
    var sessionStatuses: [SessionStatus] = []
    var isLoading = false
    var errorMessage: String?
    var pendingActions: [ShoppingAction] = []

    let martmart = MartMartClient()
    let pi = PiClient()
    let catalogStore = CatalogStore()
}
