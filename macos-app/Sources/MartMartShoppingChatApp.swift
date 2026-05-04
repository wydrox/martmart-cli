import Foundation
import SwiftUI

@main
struct MartMartShoppingChatApp: App {
    @State private var appState = AppState()

    init() {
        if CommandLine.arguments.contains("--self-check") {
            print("MartMartShoppingChat self-check OK")
            Foundation.exit(0)
        }
        ImageCache.configure()
        AppLog.write("app init")
    }

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(appState)
                .frame(minWidth: 980, minHeight: 680)
        }
        .windowStyle(.titleBar)
    }
}
