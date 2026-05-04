import Foundation
@testable import MartMartShoppingChat
import Testing

@Suite("Fixture decoding")
struct FixtureDecodingTests {
    @Test func decodesReservationSlotsFixture() throws {
        let data = try fixture("reservation-slots.response.json")
        let slots = try ReservationSlotMapper.decodeSlots(from: data)
        #expect(slots.count == 2)
        #expect(slots.first?.date == "2026-05-05")
        #expect(slots.first?.from == "10:00")
        #expect(slots.first?.to == "12:00")
    }

    @Test func decodesCheckoutPreviewFixture() throws {
        let data = try fixture("checkout-preview.response.json")
        let preview = try CheckoutMapper.decodePreview(from: data)
        #expect(preview.provider == .frisco)
        #expect(preview.cartID == "cart-1")
        #expect(preview.itemCount == 6)
        #expect(preview.readyToFinalize)
        #expect(preview.total?.display == "33.93 PLN")
    }

    @Test func decodesCheckoutFinalizeFixture() throws {
        let data = try fixture("checkout-finalize.response.json")
        let result = try CheckoutMapper.decodeFinalizeResult(from: data)
        #expect(result.status == "placed")
        #expect(result.orderID == "ord-123")
        #expect(result.action == nil)
    }

    @Test func decodesOrdersFixture() throws {
        let data = try fixture("orders-list.response.json")
        let orders = try OrderMapper.decodeList(from: data, provider: .frisco)
        #expect(orders.count == 1)
        #expect(orders[0].id == "ord-123")
        #expect(orders[0].total?.display == "33.93 PLN")
    }

    @Test func decodesProductSearchFixture() throws {
        let data = try fixture("products-search.response.json")
        let root = try JSONSerialization.jsonObject(with: data) as! [String: Any]
        let rows = root["products"] as! [[String: Any]]
        let friscoData = try JSONSerialization.data(withJSONObject: ["products": [["product": rows[0]]]])
        let delioData = try JSONSerialization.data(withJSONObject: ["data": ["productSearch": ["results": [rows[1]]]]])
        let frisco = try ProductSearchMapper.decodeProducts(from: friscoData, provider: .frisco)
        let delio = try ProductSearchMapper.decodeProducts(from: delioData, provider: .delio)
        #expect(frisco.first?.provider == .frisco)
        #expect(delio.first?.provider == .delio)
    }

    @Test func redactsMartMartSecretsFromErrors() {
        let raw = "Cookie: idToken=secret; refreshToken=secret2 Authorization: Bearer abc"
        let redacted = raw.redactedForDisplay
        #expect(!redacted.contains("secret"))
        #expect(!redacted.contains("abc"))
    }

    private func fixture(_ name: String) throws -> Data {
        let url = Bundle.module.url(forResource: name, withExtension: nil, subdirectory: "Fixtures")!
        return try Data(contentsOf: url)
    }
}
