import Testing
@testable import MartMartShoppingChat

@Suite("PiClient provider targeting")
struct PiClientTests {
    @Test func usesSelectedProviderForPlainSearch() async {
        let response = await PiClient().respond(to: "śmietana 36", provider: .delio)
        #expect(response.actions.map(\.provider) == [.delio])
    }

    @Test func comparesBothProvidersWhenAsked() async {
        let response = await PiClient().respond(to: "gdzie taniej cola zero", provider: .frisco)
        #expect(response.actions.map(\.provider) == [.frisco, .delio])
    }
}
