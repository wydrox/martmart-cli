import Foundation

extension ShoppingAction {
    var commandArguments: [String] {
        let providerArgs = provider.map { ["--provider", $0.rawValue] } ?? []
        switch kind {
        case .searchProducts:
            var args = providerArgs + ["products", "search", "--search", arguments["query"] ?? "", "--page-size", "20"]
            if provider == .delio {
                args += ["--lat", arguments["lat"] ?? "52.2297", "--long", arguments["long"] ?? "21.0122"]
            }
            return args
        case .addToCart:
            return providerArgs + ["cart", "add", "--product-id", arguments["product_id"] ?? "", "--quantity", arguments["quantity"] ?? "1"]
        case .showCart:
            return providerArgs + ["cart", "show"]
        case .checkoutPreview:
            return providerArgs + ["checkout", "preview"]
        case .checkoutFinalize:
            return providerArgs + ["checkout", "finalize", "--confirm"]
        }
    }
}
