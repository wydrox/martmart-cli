import SwiftUI

struct AppSidebarButton: View {
    let tab: AppState.Tab
    let isSelected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 10) {
                Image(systemName: tab.systemImage)
                    .frame(width: 18)
                Text(tab.rawValue)
                    .font(.callout.weight(isSelected ? .semibold : .regular))
                Spacer()
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 10)
            .foregroundStyle(isSelected ? Color.primary : Color.secondary)
            .background(isSelected ? Color.accentColor.opacity(0.14) : Color.clear, in: RoundedRectangle(cornerRadius: 12))
            .overlay(alignment: .leading) {
                if isSelected {
                    Capsule().fill(Color.accentColor).frame(width: 3).padding(.vertical, 8)
                }
            }
        }
        .buttonStyle(.plain)
    }
}

extension AppState.Tab {
    var systemImage: String {
        switch self {
        case .chat: "bubble.left.and.bubble.right"
        case .cart: "cart"
        case .checkout: "creditcard"
        case .orders: "shippingbox"
        case .history: "clock.arrow.circlepath"
        case .session: "person.crop.circle.badge.checkmark"
        }
    }

    var subtitle: String {
        switch self {
        case .chat: "Rozmowa i szybkie akcje"
        case .cart: "Koszyk aktualnego sklepu"
        case .checkout: "Slot, preview i płatność"
        case .orders: "Historia zamówień"
        case .history: "Lokalny katalog SQLite"
        case .session: "Status Frisco / Delio"
        }
    }
}

struct Surface<Content: View>: View {
    var padding: CGFloat = 16
    @ViewBuilder var content: Content

    var body: some View {
        content
            .padding(padding)
            .background(.background.opacity(0.72), in: RoundedRectangle(cornerRadius: 18, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).stroke(.quaternary))
    }
}

struct StatusPill: View {
    var text: String
    var systemImage: String
    var tint: Color = .secondary

    var body: some View {
        Label(text, systemImage: systemImage)
            .font(.caption.weight(.medium))
            .foregroundStyle(tint)
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(tint.opacity(0.10), in: Capsule())
    }
}

struct EmptyStateView: View {
    var title: String
    var subtitle: String
    var systemImage: String

    var body: some View {
        VStack(spacing: 10) {
            Image(systemName: systemImage)
                .font(.system(size: 34, weight: .semibold))
                .foregroundStyle(.tertiary)
            Text(title).font(.headline)
            Text(subtitle).font(.callout).foregroundStyle(.secondary).multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding()
    }
}
