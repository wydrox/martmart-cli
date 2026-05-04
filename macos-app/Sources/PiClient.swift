import Foundation

struct PiClient: Sendable {
    var executableURL: URL
    var timeoutSeconds: TimeInterval

    init(executableURL: URL? = nil, timeoutSeconds: TimeInterval = 75) {
        if let executableURL {
            self.executableURL = executableURL
        } else {
            self.executableURL = PiClient.findPiExecutable()
        }
        self.timeoutSeconds = timeoutSeconds
    }

    func respond(to prompt: String, provider: Provider) async -> PiResponse {
        do {
            return try await respondWithRealPi(to: prompt, provider: provider)
        } catch {
            AppLog.write("pi orchestration failed; falling back: \(error.localizedDescription)")
            var fallback = heuristicResponse(to: prompt, provider: provider)
            fallback.text = "Pi headless nie odpowiedział, więc używam awaryjnego lokalnego planu. \(fallback.text)"
            return fallback
        }
    }

    private func respondWithRealPi(to prompt: String, provider: Provider) async throws -> PiResponse {
        guard FileManager.default.isExecutableFile(atPath: executableURL.path) else {
            throw PiClientError.piNotFound(executableURL.path)
        }
        let orchestrationPrompt = Self.prompt(userPrompt: prompt, provider: provider)
        let output = try await runPi(arguments: [
            "--print",
            "--no-session",
            "--thinking", "minimal",
            "--mode", "text",
            orchestrationPrompt
        ])
        let dto = try PiResponseDTO.parse(from: output)
        return dto.toPiResponse(defaultProvider: provider)
    }

    private func runPi(arguments: [String]) async throws -> String {
        let box = ProcessBox()
        return try await withThrowingTaskGroup(of: String.self) { group in
            group.addTask {
                try await withTaskCancellationHandler {
                    try await withCheckedThrowingContinuation { continuation in
                        let process = Process()
                        box.process = process
                        process.executableURL = executableURL
                        if executableURL.path == "/usr/bin/env" {
                            process.arguments = ["pi"] + arguments
                        } else {
                            process.arguments = arguments
                        }
                        let output = Pipe()
                        let error = Pipe()
                        process.standardOutput = output
                        process.standardError = error
                        AppLog.write("pi start: \(([executableURL.path] + arguments).joined(separator: " "))")
                        process.terminationHandler = { process in
                            let data = output.fileHandleForReading.readDataToEndOfFile()
                            let errData = error.fileHandleForReading.readDataToEndOfFile()
                            let stdout = String(data: data, encoding: .utf8) ?? ""
                            let stderr = String(data: errData, encoding: .utf8) ?? ""
                            AppLog.write("pi exit status=\(process.terminationStatus) stdout=\(data.count)B stderr=\(errData.count)B")
                            if process.terminationStatus == 0 {
                                continuation.resume(returning: stdout)
                            } else if box.wasCancelled {
                                continuation.resume(throwing: CancellationError())
                            } else {
                                continuation.resume(throwing: PiClientError.commandFailed(stderr.redactedForDisplay.isEmpty ? stdout.redactedForDisplay : stderr.redactedForDisplay))
                            }
                            box.process = nil
                        }
                        do {
                            try process.run()
                        } catch {
                            AppLog.write("pi launch failed: \(error.localizedDescription)")
                            continuation.resume(throwing: error)
                            box.process = nil
                        }
                    }
                } onCancel: {
                    box.cancel()
                }
            }
            group.addTask {
                try await Task.sleep(for: .seconds(timeoutSeconds))
                box.cancel()
                throw PiClientError.timeout
            }
            let first = try await group.next() ?? ""
            group.cancelAll()
            return first
        }
    }

    private func heuristicResponse(to prompt: String, provider: Provider) -> PiResponse {
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
        let targetProviders = shouldCompareProviders(normalized) ? Provider.allCases : [provider]
        let actions = targetProviders.map { target in
            ShoppingAction(kind: .searchProducts, provider: target, title: "Szukaj w \(target.displayName): \(query)", arguments: ["query": query], requiresExplicitApproval: false)
        }
        let providerText = targetProviders.map(\.displayName).joined(separator: " i ")
        return PiResponse(text: "Szukam przez MartMart w: \(providerText). Wyniki pokażę jako karty produktów.", actions: actions)
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

    private static func findPiExecutable() -> URL {
        let candidates = [
            "/opt/homebrew/bin/pi",
            "/usr/local/bin/pi",
            "/usr/bin/env"
        ]
        let fm = FileManager.default
        if let path = candidates.first(where: { fm.isExecutableFile(atPath: $0) && $0 != "/usr/bin/env" }) {
            return URL(fileURLWithPath: path)
        }
        return URL(fileURLWithPath: "/usr/bin/env")
    }

    private static func prompt(userPrompt: String, provider: Provider) -> String {
        """
        You are pi orchestrating grocery shopping inside a lightweight macOS app.
        Use MartMart MCP/tools headlessly when useful, especially product search, cart, checkout preview, session status.
        Current selected provider is: \(provider.rawValue). Only use this provider unless the user explicitly asks to compare shops or asks where something is cheaper.
        If Delio needs coordinates, use Warsaw center lat=52.2297 long=21.0122 unless a saved address is available.

        User request: \(userPrompt)

        Return STRICT JSON only, no markdown:
        {
          "text": "short Polish answer for the user",
          "product_cards": [
            {
              "id": "provider product id",
              "provider": "frisco|delio",
              "sku": "optional sku",
              "name": "product name",
              "brand": "optional brand",
              "image_url": "optional image url",
              "price": {"amount": 1.23, "currency": "PLN"},
              "promo_price": {"amount": 1.11, "currency": "PLN"},
              "promo_condition": "optional condition",
              "deposit": {"amount": 0.50, "currency": "PLN"},
              "measure_display": "optional pack size",
              "available": true,
              "available_quantity": 10
            }
          ],
          "actions": [
            {"kind":"searchProducts|addToCart|showCart|checkoutPreview|checkoutFinalize", "provider":"frisco|delio", "title":"short label", "arguments":{}, "requires_explicit_approval": true}
          ]
        }
        For product search requests, prefer returning product_cards from actual MartMart results and no extra search action unless you could not call MartMart.
        For add-to-cart or checkout finalize, require explicit approval by returning an action instead of executing final purchase.
        """
    }
}

enum PiClientError: LocalizedError {
    case piNotFound(String)
    case commandFailed(String)
    case timeout
    case invalidJSON

    var errorDescription: String? {
        switch self {
        case .piNotFound(let path): "pi executable not found at \(path)"
        case .commandFailed(let message): message
        case .timeout: "pi orchestration timed out"
        case .invalidJSON: "pi did not return valid orchestration JSON"
        }
    }
}

private struct PiResponseDTO: Decodable {
    var text: String
    var actions: [ActionDTO]?
    var productCards: [ProductCardDTO]?

    enum CodingKeys: String, CodingKey {
        case text
        case actions
        case productCards = "product_cards"
    }

    static func parse(from output: String) throws -> PiResponseDTO {
        let trimmed = output.trimmingCharacters(in: .whitespacesAndNewlines)
        let jsonText: String
        if let start = trimmed.firstIndex(of: "{"), let end = trimmed.lastIndex(of: "}"), start <= end {
            jsonText = String(trimmed[start...end])
        } else {
            throw PiClientError.invalidJSON
        }
        guard let data = jsonText.data(using: .utf8) else { throw PiClientError.invalidJSON }
        return try JSONDecoder().decode(PiResponseDTO.self, from: data)
    }

    func toPiResponse(defaultProvider: Provider) -> PiResponse {
        PiResponse(
            text: text,
            actions: (actions ?? []).map { $0.toAction(defaultProvider: defaultProvider) },
            productCards: (productCards ?? []).compactMap { $0.toProductCard() },
            usedRealPi: true
        )
    }
}

private struct ActionDTO: Decodable {
    var kind: String
    var provider: Provider?
    var title: String
    var arguments: [String: String]?
    var requiresExplicitApproval: Bool?

    enum CodingKeys: String, CodingKey {
        case kind, provider, title, arguments
        case requiresExplicitApproval = "requires_explicit_approval"
    }

    func toAction(defaultProvider: Provider) -> ShoppingAction {
        ShoppingAction(
            kind: ShoppingAction.Kind(rawValue: kind) ?? .searchProducts,
            provider: provider ?? defaultProvider,
            title: title,
            arguments: arguments ?? [:],
            requiresExplicitApproval: requiresExplicitApproval ?? true
        )
    }
}

private struct ProductCardDTO: Decodable {
    var id: String
    var provider: Provider
    var sku: String?
    var name: String
    var brand: String?
    var imageURL: URL?
    var price: MoneyDTO
    var promoPrice: MoneyDTO?
    var promoCondition: String?
    var deposit: MoneyDTO?
    var measureDisplay: String?
    var available: Bool?
    var availableQuantity: Int?

    enum CodingKeys: String, CodingKey {
        case id, provider, sku, name, brand, price, deposit, available
        case imageURL = "image_url"
        case promoPrice = "promo_price"
        case promoCondition = "promo_condition"
        case measureDisplay = "measure_display"
        case availableQuantity = "available_quantity"
    }

    func toProductCard() -> ProductCard {
        ProductCard(
            id: id,
            provider: provider,
            sku: sku,
            name: name,
            brand: brand,
            imageURL: imageURL,
            price: price.money,
            promoPrice: promoPrice?.money,
            promoCondition: promoCondition,
            deposit: deposit?.money,
            measureDisplay: measureDisplay,
            available: available ?? true,
            availableQuantity: availableQuantity
        )
    }
}

private struct MoneyDTO: Decodable {
    var amount: Decimal
    var currency: String
    var money: Money { Money(amount: amount, currency: currency) }
}
