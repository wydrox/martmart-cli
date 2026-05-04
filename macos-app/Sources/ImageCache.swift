import Foundation

enum ImageCache {
    static func configure() {
        URLCache.shared = URLCache(
            memoryCapacity: 32 * 1024 * 1024,
            diskCapacity: 128 * 1024 * 1024,
            directory: URL(fileURLWithPath: NSTemporaryDirectory()).appending(path: "MartMartShoppingChatImageCache")
        )
    }
}
