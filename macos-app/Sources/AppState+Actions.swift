import Foundation

@MainActor
extension AppState {
    func handleUserPrompt(_ text: String) async {
        messages.append(ChatMessage(role: .user, text: text))
        isLoading = true
        let response = await pi.respond(to: text, provider: selectedProvider)
        let piBadge = response.usedRealPi ? "pi: " : "fallback: "
        messages.append(ChatMessage(role: .assistant, text: piBadge + response.text))
        pendingActions = response.actions.filter(\.requiresExplicitApproval)

        if !response.productCards.isEmpty {
            productCards = response.productCards.sorted { lhs, rhs in
                let l = NSDecimalNumber(decimal: lhs.promoPrice?.amount ?? lhs.price.amount).doubleValue
                let r = NSDecimalNumber(decimal: rhs.promoPrice?.amount ?? rhs.price.amount).doubleValue
                return l < r
            }
        } else if response.actions.contains(where: { $0.kind == .searchProducts }) {
            productCards = []
        }
        let automatic = response.actions.filter { !$0.requiresExplicitApproval }
        for action in automatic {
            await execute(action)
        }
        isLoading = false
    }

    func execute(_ action: ShoppingAction) async {
        AppLog.write("execute action kind=\(action.kind.rawValue) provider=\(action.provider?.rawValue ?? "none")")
        do {
            switch action.kind {
            case .searchProducts:
                try await executeSearch(action)
            case .addToCart:
                try await executeAddToCart(action)
            case .showCart:
                _ = try await martmart.runJSON(arguments: action.commandArguments)
                messages.append(ChatMessage(role: .assistant, text: "Odświeżyłem koszyk. Mapowanie pełnego koszyka do UI jest kolejnym krokiem."))
            case .checkoutPreview:
                let data = try await martmart.runJSON(arguments: action.commandArguments)
                checkoutPreview = try CheckoutMapper.decodePreview(from: data)
                selectedTab = .checkout
                messages.append(ChatMessage(role: .assistant, text: "Pobrałem checkout preview. Sprawdź zakładkę Checkout."))
            case .checkoutFinalize:
                let data = try await martmart.runJSON(arguments: action.commandArguments)
                checkoutResult = try CheckoutMapper.decodeFinalizeResult(from: data)
                selectedTab = .checkout
                messages.append(ChatMessage(role: .assistant, text: "Wynik finalizacji: \(checkoutResult?.status ?? "unknown")."))
            }
        } catch {
            let message = ShoppingErrorMapper.userMessage(for: error, action: action)
            errorMessage = message
            messages.append(ChatMessage(role: .assistant, text: message))
        }
    }

    private func executeSearch(_ action: ShoppingAction) async throws {
        guard let provider = action.provider else { return }
        let data = try await martmart.runJSON(arguments: action.commandArguments)
        let cards = try ProductSearchMapper.decodeProducts(from: data, provider: provider)
        let existingOtherProviders = productCards.filter { $0.provider != provider }
        productCards = (existingOtherProviders + cards).sorted { lhs, rhs in
            let l = NSDecimalNumber(decimal: lhs.promoPrice?.amount ?? lhs.price.amount).doubleValue
            let r = NSDecimalNumber(decimal: rhs.promoPrice?.amount ?? rhs.price.amount).doubleValue
            return l < r
        }
        messages.append(ChatMessage(role: .assistant, text: "\(provider.displayName): znalazłem \(cards.count) produktów i zaktualizowałem karty po prawej."))
    }

    private func executeAddToCart(_ action: ShoppingAction) async throws {
        _ = try await martmart.runJSON(arguments: action.commandArguments)
        messages.append(ChatMessage(role: .assistant, text: "Dodałem produkt do koszyka."))
    }
}

enum ShoppingErrorMapper {
    static func userMessage(for error: Error, action: ShoppingAction) -> String {
        let raw = error.localizedDescription.redactedForDisplay
        let lower = raw.lowercased()
        if lower.contains("coordinates") || lower.contains("cordinates") || lower.contains("lat") || lower.contains("long") {
            return "Delio wymaga lokalizacji. Użyłem domyślnie Warszawy centrum; jeśli nadal widzisz ten błąd, odśwież sesję Delio albo ustaw adres dostawy w sklepie. Szczegóły: \(raw)"
        }
        if lower.contains("401") || lower.contains("unauthorized") || lower.contains("unauthenticated") {
            return "Sesja \(action.provider?.displayName ?? "sklepu") wygasła. Przejdź do Session i odśwież/login, potem ponów akcję."
        }
        return "Nie udało się wykonać \(action.title): \(raw)"
    }
}
