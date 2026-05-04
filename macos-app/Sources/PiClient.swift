import Foundation

struct PiClient: Sendable {
    func respond(to prompt: String, provider: Provider) async -> PiResponse {
        let normalized = prompt.lowercased()

        if normalized.contains("checkout") || normalized.contains("zamów") || normalized.contains("zapłać") {
            return PiResponse(
                text: "Mogę przygotować checkout preview. Finalizacja będzie wymagała osobnego kliknięcia „Zamów i zapłać”.",
                actions: [ShoppingAction(kind: .checkoutPreview, provider: provider, title: "Pobierz checkout preview", arguments: [:], requiresExplicitApproval: false)]
            )
        }

        if normalized.contains("koszyk") {
            return PiResponse(
                text: "Sprawdzę aktualny koszyk w wybranym sklepie.",
                actions: [ShoppingAction(kind: .showCart, provider: provider, title: "Odśwież koszyk", arguments: [:], requiresExplicitApproval: false)]
            )
        }

        let query = prompt.trimmingCharacters(in: .whitespacesAndNewlines)
        let targetProviders: [Provider]
        if shouldCompareProviders(normalized) {
            targetProviders = Provider.allCases
        } else {
            targetProviders = [provider]
        }
        let actions = targetProviders.map { target in
            ShoppingAction(kind: .searchProducts, provider: target, title: "Szukaj w \(target.displayName): \(query)", arguments: ["query": query], requiresExplicitApproval: false)
        }
        let providerText = targetProviders.map(\.displayName).joined(separator: " i ")
        return PiResponse(
            text: "Szukam przez MartMart w: \(providerText). Wyniki pokażę jako karty produktów.",
            actions: actions
        )
    }

    private func shouldCompareProviders(_ normalized: String) -> Bool {
        normalized.contains("porówn") ||
            normalized.contains("taniej") ||
            normalized.contains("najtań") ||
            normalized.contains("gdzie") ||
            normalized.contains("frisco i delio") ||
            normalized.contains("delio i frisco") ||
            normalized.contains("oba") ||
            normalized.contains("obyd")
    }
}
