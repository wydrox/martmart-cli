import Foundation

enum CartErrorMapper {
    static func userMessage(for error: Error) -> String {
        let text = error.localizedDescription.lowercased()
        if text.contains("unauthorized") || text.contains("not authenticated") || text.contains("401") || text.contains("session") {
            return "Sesja sklepu wygasła. Sprawdź zakładkę Session i odśwież/login przez MartMart."
        }
        if text.contains("not available") || text.contains("unavailable") || text.contains("niedost") {
            return "Produkt jest niedostępny. Odśwież wyniki i wybierz zamiennik."
        }
        if text.contains("stock") || text.contains("quantity") || text.contains("availablequantity") {
            return "Brak wystarczającego stocku dla wybranej ilości. Zmniejsz ilość albo wybierz inny produkt."
        }
        if text.contains("price") || text.contains("promotion") || text.contains("promo") || text.contains("cena") {
            return "Cena lub promocja zmieniła się. Odśwież koszyk przed checkoutem."
        }
        return error.localizedDescription.redactedForDisplay
    }
}
