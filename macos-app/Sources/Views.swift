import SwiftUI

struct ChatView: View {
    @Environment(AppState.self) private var appState
    @State private var draft = ""

    var body: some View {
        Surface(padding: 0) {
            VStack(spacing: 0) {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 12) {
                        ForEach(appState.messages) { message in
                            MessageBubble(message: message)
                        }
                    }
                    .padding(18)
                }
                Divider()
                HStack(alignment: .bottom, spacing: 10) {
                    TextField("Co kupujemy? np. „porównaj colę zero 330 ml”", text: $draft, axis: .vertical)
                        .textFieldStyle(.plain)
                        .lineLimit(1...4)
                        .padding(12)
                        .background(.quaternary, in: RoundedRectangle(cornerRadius: 12))
                    Button {
                        Task { await send() }
                    } label: {
                        Label("Wyślij", systemImage: "arrow.up.circle.fill")
                    }
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.return, modifiers: .command)
                    .disabled(draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
                .padding(14)
            }
        }
    }

    private func send() async {
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        draft = ""
        await appState.handleUserPrompt(text)
    }
}

struct PendingActionsView: View {
    @Environment(AppState.self) private var appState
    var actions: [ShoppingAction]

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Proponowane akcje").font(.headline)
            ForEach(actions) { action in
                HStack {
                    VStack(alignment: .leading) {
                        Text(action.title)
                        if let provider = action.provider {
                            Text(provider.displayName).font(.caption).foregroundStyle(.secondary)
                        }
                    }
                    Spacer()
                    Button("Odrzuć") { appState.pendingActions.removeAll { $0.id == action.id } }
                    Button("Wykonaj") { Task { await approve(action) } }
                        .buttonStyle(.borderedProminent)
                }
                .padding(10)
                .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 10))
            }
        }
        .padding(12)
        .background(.quaternary, in: RoundedRectangle(cornerRadius: 14))
    }

    private func approve(_ action: ShoppingAction) async {
        appState.pendingActions.removeAll { $0.id == action.id }
        appState.messages.append(ChatMessage(role: .system, text: "Zaakceptowano: \(action.title). Wykonuję przez MartMart headless."))
        await appState.execute(action)
    }
}

struct ProductRail: View {
    var products: [ProductCard]

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(alignment: .top, spacing: 12) {
                ForEach(products) { product in
                    ProductCardView(product: product)
                        .frame(width: 240)
                }
            }
        }
    }
}

struct ProductCardView: View {
    @Environment(AppState.self) private var appState
    var product: ProductCard

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            AsyncImage(url: product.imageURL) { phase in
                switch phase {
                case .success(let image): image.resizable().scaledToFit()
                case .failure: Image(systemName: "photo").font(.title2).foregroundStyle(.secondary)
                case .empty: ProgressView().scaleEffect(0.7)
                @unknown default: EmptyView()
                }
            }
            .frame(width: 62, height: 62)
            .background(.quaternary, in: RoundedRectangle(cornerRadius: 12))

            VStack(alignment: .leading, spacing: 6) {
                HStack(alignment: .top) {
                    Text(product.name)
                        .font(.callout.weight(.semibold))
                        .lineLimit(2)
                    Spacer(minLength: 4)
                    Text(product.provider.displayName)
                        .font(.caption2.weight(.medium))
                        .foregroundStyle(.secondary)
                }
                HStack(alignment: .firstTextBaseline, spacing: 6) {
                    Text((product.promoPrice ?? product.price).display)
                        .font(.headline.bold())
                    if product.promoPrice != nil {
                        Text(product.price.display)
                            .font(.caption)
                            .strikethrough()
                            .foregroundStyle(.secondary)
                    }
                }
                HStack(spacing: 8) {
                    if let deposit = product.deposit {
                        Text("kaucja +\(deposit.display)")
                    }
                    if let condition = product.promoCondition {
                        Text(condition)
                    }
                }
                .font(.caption2)
                .foregroundStyle(.secondary)
                Button("Dodaj do koszyka") { Task { await addToCart() } }
                    .buttonStyle(.bordered)
                    .controlSize(.small)
                    .disabled(!product.available)
            }
        }
        .padding(10)
        .background(.background, in: RoundedRectangle(cornerRadius: 14))
        .overlay(RoundedRectangle(cornerRadius: 14).stroke(.quaternary))
    }

    private func addToCart() async {
        let action = ShoppingAction(
            kind: .addToCart,
            provider: product.provider,
            title: "Dodaj do koszyka: \(product.name)",
            arguments: ["product_id": product.id, "quantity": "1"],
            requiresExplicitApproval: true
        )
        await appState.execute(action)
    }
}

struct MessageBubble: View {
    var message: ChatMessage

    var body: some View {
        Text(message.text)
            .padding(10)
            .background(message.role == .user ? Color.accentColor.opacity(0.16) : Color.secondary.opacity(0.10), in: RoundedRectangle(cornerRadius: 12))
            .frame(maxWidth: 680, alignment: message.role == .user ? .trailing : .leading)
            .frame(maxWidth: .infinity, alignment: message.role == .user ? .trailing : .leading)
    }
}

struct CartView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Provider: \(appState.selectedProvider.displayName)")
                    .font(.headline)
                Spacer()
                Button("Odśwież") { Task { await refreshCart() } }
            }
            .padding()
            Divider()
            List {
                ForEach(appState.cart.items) { item in
                    HStack {
                        Text(item.product.name)
                        Spacer()
                        HStack(spacing: 6) {
                            Button("−") { Task { await setQuantity(item, quantity: max(0, item.quantity - 1)) } }
                            Text("×\(item.quantity)").frame(width: 36)
                            Button("+") { Task { await setQuantity(item, quantity: item.quantity + 1) } }
                        }
                        Text(item.lineTotal.display)
                            .frame(width: 90, alignment: .trailing)
                        Button(role: .destructive) { Task { await remove(item) } } label: {
                            Image(systemName: "trash")
                        }
                    }
                }
                if appState.cart.items.isEmpty {
                    ContentUnavailableView("Koszyk pusty", systemImage: "cart", description: Text("Dodaj produkty z chatu lub wyszukiwarki."))
                }
            }
            if let total = appState.cart.grandTotal {
                Divider()
                HStack {
                    Spacer()
                    Text("Razem: \(total.display)").font(.title3.bold())
                }
                .padding()
            }
        }
        .navigationTitle("Koszyk")
    }

    private func refreshCart() async {
        await runCartCommand(["cart", "show"], success: "Odświeżono koszyk JSON. Następny krok: mapowanie na CartSummary.")
    }

    private func setQuantity(_ item: CartItem, quantity: Int) async {
        await runCartCommand(["cart", "add", "--product-id", item.product.id, "--quantity", String(quantity)], success: "Zaktualizowano ilość: \(item.product.name).")
    }

    private func remove(_ item: CartItem) async {
        await runCartCommand(["cart", "remove", "--product-id", item.product.id], success: "Usunięto: \(item.product.name).")
    }

    private func runCartCommand(_ args: [String], success: String) async {
        do {
            _ = try await appState.martmart.runJSON(arguments: ["--provider", appState.selectedProvider.rawValue] + args)
            appState.messages.append(ChatMessage(role: .system, text: success))
        } catch {
            let message = CartErrorMapper.userMessage(for: error)
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: message))
        }
    }
}

struct CheckoutView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                ReservationPickerView()
                Divider()
                CheckoutPreviewView()
            }
            .padding()
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .navigationTitle("Checkout")
    }
}

struct ReservationPickerView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                VStack(alignment: .leading) {
                    Text("Dostawa").font(.title2.bold())
                    Text("MVP używa zapisanego adresu z MartMart; później dodamy wybór adresu z listy.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                Button("Pobierz sloty") { Task { await loadSlots() } }
            }

            if appState.reservationSlots.isEmpty {
                Text("Brak pobranych slotów.")
                    .foregroundStyle(.secondary)
            } else {
                LazyVGrid(columns: [GridItem(.adaptive(minimum: 180), spacing: 10)], spacing: 10) {
                    ForEach(appState.reservationSlots) { slot in
                        Button {
                            appState.selectedReservationSlot = slot
                        } label: {
                            VStack(alignment: .leading, spacing: 4) {
                                Text(slot.date).font(.caption).foregroundStyle(.secondary)
                                Text(slot.displayTime).font(.headline)
                                Text(slot.deliveryMethod ?? "Van").font(.caption)
                                if let price = slot.price { Text(price.display).font(.caption2) }
                            }
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(10)
                            .background(appState.selectedReservationSlot?.id == slot.id ? Color.accentColor.opacity(0.18) : Color.secondary.opacity(0.08), in: RoundedRectangle(cornerRadius: 10))
                        }
                        .buttonStyle(.plain)
                        .disabled(!slot.available)
                    }
                }
                Button("Zarezerwuj wybrany slot") { Task { await reserveSelectedSlot() } }
                    .buttonStyle(.borderedProminent)
                    .disabled(appState.selectedReservationSlot == nil)
            }
        }
    }

    private func loadSlots() async {
        do {
            let data = try await appState.martmart.runJSON(arguments: ["--provider", appState.selectedProvider.rawValue, "reservation", "slots", "--days", "3"])
            appState.reservationSlots = try ReservationSlotMapper.decodeSlots(from: data)
            appState.messages.append(ChatMessage(role: .system, text: "Pobrano \(appState.reservationSlots.count) slotów dostawy."))
        } catch {
            let message = error.localizedDescription.redactedForDisplay
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: "Nie udało się pobrać slotów: \(message)"))
        }
    }

    private func reserveSelectedSlot() async {
        guard let slot = appState.selectedReservationSlot else { return }
        do {
            _ = try await appState.martmart.runJSON(arguments: [
                "--provider", appState.selectedProvider.rawValue,
                "reservation", "reserve",
                "--date", slot.date,
                "--from-time", slot.from,
                "--to-time", slot.to
            ])
            appState.messages.append(ChatMessage(role: .system, text: "Zarezerwowano slot \(slot.date) \(slot.displayTime)."))
        } catch {
            let message = error.localizedDescription.redactedForDisplay
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: "Nie udało się zarezerwować slotu: \(message)"))
        }
    }
}

struct CheckoutPreviewView: View {
    @Environment(AppState.self) private var appState
    @State private var showFinalizeConfirmation = false

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Checkout preview").font(.title2.bold())
                Spacer()
                Button("Pobierz preview") { Task { await loadPreview() } }
            }
            if let preview = appState.checkoutPreview {
                Grid(alignment: .leading, horizontalSpacing: 18, verticalSpacing: 8) {
                    GridRow { Text("Pozycji").foregroundStyle(.secondary); Text("\(preview.itemCount)") }
                    GridRow { Text("Suma").foregroundStyle(.secondary); Text(preview.total?.display ?? "—") }
                    GridRow { Text("Dostawa").foregroundStyle(.secondary); Text(deliveryText(preview.reservation)) }
                    GridRow { Text("Płatność").foregroundStyle(.secondary); Text(paymentText(preview.payment)) }
                    GridRow { Text("Status").foregroundStyle(.secondary); Text(preview.readyToFinalize ? "Gotowe do finalizacji" : "Wymaga uwagi") }
                }
                if !preview.issues.isEmpty {
                    VStack(alignment: .leading, spacing: 6) {
                        Text("Ostrzeżenia").font(.headline)
                        ForEach(preview.issues) { issue in
                            Label(issue.message.isEmpty ? issue.code : issue.message, systemImage: "exclamationmark.triangle")
                                .foregroundStyle(.orange)
                        }
                    }
                }
                Button("Zamów i zapłać") { showFinalizeConfirmation = true }
                    .buttonStyle(.borderedProminent)
                    .disabled(!preview.readyToFinalize)
                    .confirmationDialog("Potwierdź finalizację", isPresented: $showFinalizeConfirmation) {
                        Button("Zamów i zapłać", role: .destructive) { Task { await finalizeCheckout() } }
                        Button("Anuluj", role: .cancel) {}
                    } message: {
                        Text("Aplikacja wyśle headless checkout przez MartMart. Przed finalizacją MartMart pobierze świeży preview i przerwie, jeśli guard się nie zgadza.")
                    }
                if let result = appState.checkoutResult {
                    CheckoutResultView(result: result)
                }
            } else {
                ContentUnavailableView("Brak preview", systemImage: "creditcard", description: Text("Najpierw pobierz checkout preview przez MartMart."))
            }
        }
    }

    private func loadPreview() async {
        do {
            let data = try await appState.martmart.runJSON(arguments: ["--provider", appState.selectedProvider.rawValue, "checkout", "preview"])
            appState.checkoutPreview = try CheckoutMapper.decodePreview(from: data)
            appState.messages.append(ChatMessage(role: .system, text: "Pobrano checkout preview."))
        } catch {
            let message = error.localizedDescription.redactedForDisplay
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: "Nie udało się pobrać checkout preview: \(message)"))
        }
    }

    private func finalizeCheckout() async {
        guard appState.checkoutPreview?.readyToFinalize == true else { return }
        do {
            let data = try await appState.martmart.runJSON(arguments: ["--provider", appState.selectedProvider.rawValue, "checkout", "finalize", "--confirm"])
            appState.checkoutResult = try CheckoutMapper.decodeFinalizeResult(from: data)
            let status = appState.checkoutResult?.status ?? "unknown"
            appState.messages.append(ChatMessage(role: .system, text: "Checkout finalize: \(status)."))
        } catch {
            let message = error.localizedDescription.redactedForDisplay
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: "Checkout przerwany: \(message)"))
        }
    }

    private func deliveryText(_ reservation: ReservationWindow?) -> String {
        guard let reservation else { return "—" }
        let range = [reservation.startsAt, reservation.endsAt].compactMap { $0 }.joined(separator: " – ")
        return range.isEmpty ? (reservation.deliveryMethod ?? "—") : range
    }

    private func paymentText(_ payment: PaymentSelection?) -> String {
        guard let payment else { return "—" }
        return [payment.method, payment.channel, payment.status].compactMap { $0 }.joined(separator: " / ")
    }
}

struct CheckoutResultView: View {
    @Environment(\.openURL) private var openURL
    var result: CheckoutFinalizeResultViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Wynik finalizacji").font(.headline)
            Label(statusText, systemImage: icon)
                .foregroundStyle(result.status == "placed" ? .green : .orange)
            if let orderID = result.orderID {
                Text("Order ID: \(orderID)").font(.caption).foregroundStyle(.secondary)
            }
            if let action = result.action {
                ActionRequiredView(action: action)
            }
        }
        .padding(12)
        .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 12))
    }

    private var statusText: String {
        switch result.status {
        case "placed": "Zamówienie złożone"
        case "requires_action": "Wymagana dodatkowa akcja"
        case "pending": "Płatność/zamówienie oczekuje"
        default: "Status: \(result.status)"
        }
    }

    private var icon: String {
        result.status == "placed" ? "checkmark.circle" : "exclamationmark.triangle"
    }
}

struct ActionRequiredView: View {
    @Environment(\.openURL) private var openURL
    var action: PaymentActionViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(action.message ?? "Dokończ autoryzację płatności.")
            Text("Typ: \(action.kind)").font(.caption).foregroundStyle(.secondary)
            if let urlText = action.url, let url = URL(string: urlText) {
                Button("Otwórz autoryzację") { openURL(url) }
                    .buttonStyle(.borderedProminent)
                Text("Po zakończeniu wróć do aplikacji i odśwież zamówienia/koszyk.")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(10)
        .background(Color.orange.opacity(0.12), in: RoundedRectangle(cornerRadius: 10))
    }
}

struct OrdersView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        Surface(padding: 0) {
            VStack(spacing: 0) {
                HStack {
                    Text("Historia zamówień").font(.headline)
                    Spacer()
                    Button("Odśwież") { Task { await loadOrders() } }
                }
                .padding()
                Divider()
                HSplitView {
                    List(appState.orders) { order in
                        Button {
                            appState.selectedOrder = order
                        } label: {
                            HStack {
                                VStack(alignment: .leading) {
                                    Text(order.id).font(.headline)
                                    Text([order.provider.displayName, order.createdAt].compactMap { $0 }.joined(separator: " · "))
                                        .foregroundStyle(.secondary)
                                }
                                Spacer()
                                Text(order.status)
                                Text(order.total?.display ?? "")
                                    .frame(width: 90, alignment: .trailing)
                            }
                            .contentShape(Rectangle())
                        }
                        .buttonStyle(.plain)
                        .listRowBackground(appState.selectedOrder?.id == order.id ? Color.accentColor.opacity(0.14) : Color.clear)
                    }
                    .frame(minWidth: 380)
                    .overlay {
                        if appState.orders.isEmpty {
                            ContentUnavailableView("Brak zamówień", systemImage: "shippingbox")
                        }
                    }

                    if let order = appState.selectedOrder {
                        OrderDetailsView(order: order)
                            .frame(minWidth: 420)
                    } else {
                        EmptyStateView(title: "Wybierz zamówienie", subtitle: "Szczegóły, płatność i dostawa pojawią się tutaj.", systemImage: "shippingbox")
                            .frame(minWidth: 420)
                    }
                }
            }
        }
    }

    private func loadOrders() async {
        do {
            let data = try await appState.martmart.runJSON(arguments: ["--provider", appState.selectedProvider.rawValue, "orders", "list", "--page-size", "20"])
            appState.orders = try OrderMapper.decodeList(from: data, provider: appState.selectedProvider)
            appState.selectedOrder = appState.orders.first
            appState.messages.append(ChatMessage(role: .system, text: "Pobrano \(appState.orders.count) zamówień."))
        } catch {
            let message = error.localizedDescription.redactedForDisplay
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: "Nie udało się pobrać zamówień: \(message)"))
        }
    }
}

struct OrderDetailsView: View {
    @Environment(AppState.self) private var appState
    var order: OrderSummary
    @State private var rawDetails = ""
    @State private var rawPayments = ""
    @State private var rawDelivery = ""
    @State private var showRaw = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                Text(order.id).font(.title2.bold())
                Grid(alignment: .leading, horizontalSpacing: 12, verticalSpacing: 8) {
                    GridRow { Text("Provider").foregroundStyle(.secondary); Text(order.provider.displayName) }
                    GridRow { Text("Status").foregroundStyle(.secondary); Text(order.status) }
                    GridRow { Text("Data").foregroundStyle(.secondary); Text(order.createdAt ?? "—") }
                    GridRow { Text("Suma").foregroundStyle(.secondary); Text(order.total?.display ?? "—") }
                }
                HStack {
                    Button("Pobierz szczegóły") { Task { await loadDetails() } }
                    Button("Płatności") { Task { await loadPayments() } }
                    Button("Dostawa") { Task { await loadDelivery() } }
                    Toggle("Raw debug", isOn: $showRaw)
                }
                if showRaw {
                    RawBlock(title: "Order", text: rawDetails)
                    RawBlock(title: "Payments", text: rawPayments)
                    RawBlock(title: "Delivery", text: rawDelivery)
                }
            }
            .padding()
            .frame(maxWidth: .infinity, alignment: .topLeading)
        }
        .navigationTitle("Order details")
    }

    private func loadDetails() async { rawDetails = await runOrderCommand(["orders", "get", "--order-id", order.id]) }
    private func loadPayments() async { rawPayments = await runOrderCommand(["orders", "payments", "--order-id", order.id]) }
    private func loadDelivery() async { rawDelivery = await runOrderCommand(["orders", "delivery", "--order-id", order.id]) }

    private func runOrderCommand(_ args: [String]) async -> String {
        do {
            let data = try await appState.martmart.runJSON(arguments: ["--provider", order.provider.rawValue] + args)
            return OrderMapper.prettyJSONString(from: data)
        } catch {
            return error.localizedDescription.redactedForDisplay
        }
    }
}

struct RawBlock: View {
    var title: String
    var text: String

    var body: some View {
        VStack(alignment: .leading) {
            Text(title).font(.headline)
            ScrollView(.horizontal) {
                Text(text.isEmpty ? "—" : text)
                    .font(.system(.caption, design: .monospaced))
                    .textSelection(.enabled)
                    .padding(8)
            }
            .frame(maxWidth: .infinity, minHeight: 80, alignment: .leading)
            .background(.quaternary, in: RoundedRectangle(cornerRadius: 8))
        }
    }
}

struct ProductHistoryView: View {
    @Environment(AppState.self) private var appState
    @State private var query = ""

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                TextField("Szukaj w lokalnym katalogu", text: $query)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { Task { await search() } }
                Button("Szukaj") { Task { await search() } }
                Button("Ostatnie") { Task { await loadRecent() } }
                if appState.selectedCatalogProduct != nil {
                    Button("Zamknij szczegóły") { appState.selectedCatalogProduct = nil }
                }
            }
            .padding()
            Divider()
            HSplitView {
                ProductHistoryList(
                    products: appState.catalogProducts,
                    selectedProduct: appState.selectedCatalogProduct,
                    select: { product in appState.selectedCatalogProduct = product }
                )
                .frame(minWidth: 420, idealWidth: 520)

                if let product = appState.selectedCatalogProduct {
                    PriceHistoryDetail(product: product)
                        .frame(minWidth: 420)
                } else {
                    ContentUnavailableView("Wybierz produkt", systemImage: "chart.line.uptrend.xyaxis", description: Text("Szczegóły historii ceny pojawią się tutaj, bez przechodzenia na osobny ekran."))
                        .frame(minWidth: 420)
                }
            }
        }
        .task { if appState.catalogProducts.isEmpty { await loadRecent() } }
        .navigationTitle("Historia")
    }

    private func loadRecent() async {
        let store = appState.catalogStore
        do {
            appState.catalogProducts = try await Task.detached { try store.recentProducts(limit: 80) }.value
            if let selected = appState.selectedCatalogProduct,
               !appState.catalogProducts.contains(where: { $0.id == selected.id }) {
                appState.selectedCatalogProduct = nil
            }
        } catch {
            appState.errorMessage = error.localizedDescription.redactedForDisplay
        }
    }

    private func search() async {
        let text = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { await loadRecent(); return }
        let store = appState.catalogStore
        do {
            appState.catalogProducts = try await Task.detached { try store.searchProducts(text, limit: 80) }.value
            appState.selectedCatalogProduct = appState.catalogProducts.first
        } catch {
            appState.errorMessage = error.localizedDescription.redactedForDisplay
        }
    }
}

struct ProductHistoryList: View {
    var products: [CatalogProduct]
    var selectedProduct: CatalogProduct?
    var select: (CatalogProduct) -> Void

    var body: some View {
        List(products) { product in
            Button {
                select(product)
            } label: {
                HStack {
                    VStack(alignment: .leading) {
                        Text(product.name).font(.headline)
                        Text([product.provider.displayName, product.brand, product.measureText].compactMap { $0 }.joined(separator: " · "))
                            .foregroundStyle(.secondary)
                    }
                    Spacer()
                    Text(product.available == false ? "niedostępny" : "")
                        .foregroundStyle(.secondary)
                    Text(product.price?.display ?? "—")
                        .frame(width: 90, alignment: .trailing)
                }
                .padding(.vertical, 4)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .listRowBackground(selectedProduct?.id == product.id ? Color.accentColor.opacity(0.14) : Color.clear)
        }
        .overlay {
            if products.isEmpty {
                ContentUnavailableView("Historia produktów", systemImage: "clock", description: Text("Wczytaj ostatnie produkty albo wyszukaj lokalnie w SQLite."))
            }
        }
    }
}

struct PriceHistoryDetail: View {
    @Environment(AppState.self) private var appState
    var product: CatalogProduct
    @State private var snapshots: [PriceSnapshot] = []

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text(product.name).font(.title2.bold())
            Grid(alignment: .leading, horizontalSpacing: 12, verticalSpacing: 8) {
                GridRow { Text("Provider").foregroundStyle(.secondary); Text(product.provider.displayName) }
                GridRow { Text("ID").foregroundStyle(.secondary); Text(product.externalID) }
                GridRow { Text("Aktualna cena").foregroundStyle(.secondary); Text(product.price?.display ?? "—") }
                GridRow { Text("Ostatnio widziany").foregroundStyle(.secondary); Text(product.lastSeenAt) }
            }
            Text("Historia cen").font(.headline)
            List(snapshots) { snapshot in
                HStack {
                    VStack(alignment: .leading) {
                        Text(snapshot.seenAt)
                        Text(snapshot.source).font(.caption).foregroundStyle(.secondary)
                    }
                    Spacer()
                    if let promo = snapshot.promoPrice {
                        Text(promo.display).bold()
                    }
                    Text(snapshot.price?.display ?? "—")
                    if snapshot.available == false {
                        Text("niedostępny").foregroundStyle(.secondary)
                    }
                }
            }
        }
        .padding()
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .task(id: product.id) { await loadSnapshots() }
    }

    private func loadSnapshots() async {
        let store = appState.catalogStore
        let productID = product.id
        do {
            snapshots = try await Task.detached { try store.snapshots(productID: productID, limit: 100) }.value
        } catch {
            appState.errorMessage = error.localizedDescription.redactedForDisplay
        }
    }
}

struct SessionStatusView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Sesje MartMart").font(.headline)
                Spacer()
                Button("Odśwież") { Task { await refresh() } }
                Button("Verify selected") { Task { await verifySelected() } }
                Button("Refresh token") { Task { await refreshTokenSelected() } }
                Button("Login selected") { Task { await loginSelected() } }
            }
            .padding()
            Divider()
            List(appState.sessionStatuses) { status in
                VStack(alignment: .leading, spacing: 6) {
                    HStack {
                        Label(status.provider.displayName, systemImage: status.authPresent ? "checkmark.circle.fill" : "xmark.circle")
                            .foregroundStyle(status.authPresent ? .green : .red)
                        Spacer()
                        Text(status.saved ? "saved" : "missing")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    Text(status.baseURL).font(.caption).foregroundStyle(.secondary)
                    if !status.userID.isEmpty { Text("user: \(status.userID)").font(.caption).foregroundStyle(.secondary) }
                    Text("token \(status.tokenSaved ? "✓" : "—") · refresh \(status.refreshTokenSaved ? "✓" : "—") · cookie \(status.cookieSaved ? "✓" : "—")")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                    if let file = status.sessionFile { Text(file).font(.caption2).foregroundStyle(.tertiary) }
                }
                .padding(.vertical, 4)
            }
            .overlay {
                if appState.sessionStatuses.isEmpty {
                    ContentUnavailableView("Status sesji", systemImage: "person.crop.circle.badge.checkmark", description: Text("Odśwież, żeby sprawdzić Frisco/Delio. Jeśli sesja wygasła, użyj login albo importu cURL w MartMart."))
                }
            }
        }
        .task { if appState.sessionStatuses.isEmpty { await refresh() } }
        .navigationTitle("Sesja")
    }

    private func refresh() async {
        do {
            let data = try await appState.martmart.runJSON(arguments: ["session", "list"])
            appState.sessionStatuses = try SessionStatusMapper.decodeList(from: data)
        } catch {
            appState.errorMessage = error.localizedDescription.redactedForDisplay
        }
    }

    private func verifySelected() async {
        do {
            _ = try await appState.martmart.runJSON(arguments: ["--provider", appState.selectedProvider.rawValue, "session", "verify"])
            appState.messages.append(ChatMessage(role: .system, text: "Sesja \(appState.selectedProvider.displayName) OK."))
            await refresh()
        } catch {
            let message = error.localizedDescription.redactedForDisplay
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: "Sesja wymaga odświeżenia: \(message). Możesz użyć Login selected albo zaimportować cURL przez MartMart CLI."))
        }
    }

    private func refreshTokenSelected() async {
        do {
            _ = try await appState.martmart.runJSON(arguments: ["--provider", appState.selectedProvider.rawValue, "session", "refresh-token"])
            appState.messages.append(ChatMessage(role: .system, text: "Odświeżono token \(appState.selectedProvider.displayName)."))
            await refresh()
        } catch {
            let message = error.localizedDescription.redactedForDisplay
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: "Refresh token nie powiódł się: \(message). Dla Delio zwykle potrzebny jest Login selected albo import cURL."))
        }
    }

    private func loginSelected() async {
        do {
            _ = try await appState.martmart.runJSON(arguments: ["--provider", appState.selectedProvider.rawValue, "session", "login", "--force"])
            appState.messages.append(ChatMessage(role: .system, text: "Zapisano sesję \(appState.selectedProvider.displayName)."))
            await refresh()
        } catch {
            let message = error.localizedDescription.redactedForDisplay
            appState.errorMessage = message
            appState.messages.append(ChatMessage(role: .assistant, text: "Login nie powiódł się: \(message). Alternatywa: skopiuj API request jako cURL i użyj `martmart --provider \(appState.selectedProvider.rawValue) session from-curl`."))
        }
    }
}
