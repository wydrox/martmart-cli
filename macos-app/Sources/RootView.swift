import SwiftUI

struct RootView: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        @Bindable var appState = appState
        HStack(spacing: 0) {
            sidebar
            Divider()
            VStack(spacing: 0) {
                topBar
                Divider()
                HStack(spacing: 16) {
                    mainContent
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                    AssistantInspector()
                        .frame(width: 320)
                }
                .padding(16)
                .background(Color(nsColor: .windowBackgroundColor))
            }
        }
    }

    private var sidebar: some View {
        VStack(alignment: .leading, spacing: 14) {
            VStack(alignment: .leading, spacing: 4) {
                Text("MartMart")
                    .font(.title2.bold())
                Text("Shopping Chat")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .padding(.horizontal, 14)
            .padding(.top, 18)

            VStack(spacing: 4) {
                ForEach(AppState.Tab.allCases) { tab in
                    AppSidebarButton(tab: tab, isSelected: appState.selectedTab == tab) {
                        appState.selectedTab = tab
                    }
                }
            }
            .padding(.horizontal, 10)

            Spacer()

            Surface(padding: 12) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Checkout safety")
                        .font(.caption.bold())
                    Text("Finalizacja zawsze wymaga świeżego preview i ręcznego kliknięcia.")
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                }
            }
            .padding(10)
        }
        .frame(width: 218)
        .background(.ultraThinMaterial)
    }

    private var topBar: some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 2) {
                Text(appState.selectedTab.rawValue)
                    .font(.title2.bold())
                Text(appState.selectedTab.subtitle)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if appState.isLoading {
                ProgressView().scaleEffect(0.72)
            }
            Picker("Provider", selection: Binding(
                get: { appState.selectedProvider },
                set: { appState.selectedProvider = $0 }
            )) {
                ForEach(Provider.allCases) { provider in
                    Text(provider.displayName).tag(provider)
                }
            }
            .pickerStyle(.segmented)
            .frame(width: 180)
            if let message = appState.errorMessage, !message.isEmpty {
                StatusPill(text: "Błąd", systemImage: "exclamationmark.triangle", tint: .orange)
                    .help(message)
            } else {
                StatusPill(text: "Local", systemImage: "bolt.horizontal", tint: .green)
            }
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 12)
        .background(.bar)
    }

    @ViewBuilder
    private var mainContent: some View {
        switch appState.selectedTab {
        case .chat:
            ChatView()
        case .cart:
            CartView()
        case .checkout:
            CheckoutView()
        case .orders:
            OrdersView()
        case .history:
            ProductHistoryView()
        case .session:
            SessionStatusView()
        }
    }
}

struct AssistantInspector: View {
    @Environment(AppState.self) private var appState

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Surface {
                VStack(alignment: .leading, spacing: 10) {
                    Text("Szybki stan")
                        .font(.headline)
                    StatusPill(text: appState.selectedProvider.displayName, systemImage: "storefront", tint: .accentColor)
                    Text("Produkty: \(appState.productCards.count) · Akcje: \(appState.pendingActions.count)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    if let error = appState.errorMessage, !error.isEmpty {
                        Text(error)
                            .font(.caption)
                            .foregroundStyle(.orange)
                            .lineLimit(4)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }

            if !appState.pendingActions.isEmpty {
                PendingActionsView(actions: appState.pendingActions)
            }

            Surface {
                VStack(alignment: .leading, spacing: 10) {
                    Text("Produkty")
                        .font(.headline)
                    if appState.productCards.isEmpty {
                        Text("Wyniki wyszukiwania pojawią się tutaj.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    } else {
                        ScrollView {
                            LazyVStack(spacing: 10) {
                                ForEach(appState.productCards) { product in
                                    ProductCardView(product: product)
                                }
                            }
                        }
                        .frame(maxHeight: 420)
                    }
                }
            }

            Spacer(minLength: 0)
        }
    }
}
