// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "MartMartShoppingChat",
    platforms: [.macOS(.v14)],
    products: [
        .executable(name: "MartMartShoppingChat", targets: ["MartMartShoppingChat"])
    ],
    targets: [
        .executableTarget(
            name: "MartMartShoppingChat",
            path: "Sources",
            linkerSettings: [.linkedLibrary("sqlite3")]
        ),
        .testTarget(
            name: "MartMartShoppingChatTests",
            dependencies: ["MartMartShoppingChat"],
            path: "Tests",
            resources: [.copy("Fixtures")]
        )
    ]
)
