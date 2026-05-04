import Foundation
import SQLite3

struct CatalogStore: Sendable {
    var databaseURL: URL

    init(databaseURL: URL? = nil) {
        if let databaseURL {
            self.databaseURL = databaseURL
        } else {
            self.databaseURL = FileManager.default.homeDirectoryForCurrentUser
                .appending(path: ".martmart-cli")
                .appending(path: "catalog.db")
        }
    }

    func recentProducts(limit: Int = 50) throws -> [CatalogProduct] {
        try queryProducts(sql: """
            SELECT id, provider, external_id, name, brand, current_price_minor, current_currency, measure_text, current_available, last_seen_at
            FROM products
            ORDER BY last_seen_at DESC
            LIMIT ?
            """, bindings: [.int(limit)])
    }

    func searchProducts(_ query: String, limit: Int = 50) throws -> [CatalogProduct] {
        let like = "%\(query)%"
        return try queryProducts(sql: """
            SELECT id, provider, external_id, name, brand, current_price_minor, current_currency, measure_text, current_available, last_seen_at
            FROM products
            WHERE name LIKE ? OR brand LIKE ? OR search_blob LIKE ?
            ORDER BY last_seen_at DESC
            LIMIT ?
            """, bindings: [.text(like), .text(like), .text(like), .int(limit)])
    }

    func snapshots(productID: Int64, limit: Int = 100) throws -> [PriceSnapshot] {
        let db = try openReadOnly()
        defer { sqlite3_close(db) }
        let sql = """
            SELECT id, seen_at, source, price_minor, promo_price_minor, available
            FROM product_snapshots
            WHERE product_id = ?
            ORDER BY seen_at DESC
            LIMIT ?
            """
        let statement = try prepare(db, sql: sql)
        defer { sqlite3_finalize(statement) }
        sqlite3_bind_int64(statement, 1, productID)
        sqlite3_bind_int(statement, 2, Int32(limit))

        var rows: [PriceSnapshot] = []
        while sqlite3_step(statement) == SQLITE_ROW {
            rows.append(PriceSnapshot(
                id: sqlite3_column_int64(statement, 0),
                seenAt: columnString(statement, 1),
                source: columnString(statement, 2),
                price: moneyFromMinor(sqlite3_column_int64(statement, 3)),
                promoPrice: sqlite3_column_type(statement, 4) == SQLITE_NULL ? nil : moneyFromMinor(sqlite3_column_int64(statement, 4)),
                available: sqlite3_column_type(statement, 5) == SQLITE_NULL ? nil : sqlite3_column_int(statement, 5) != 0
            ))
        }
        return rows
    }

    private func queryProducts(sql: String, bindings: [SQLiteBinding]) throws -> [CatalogProduct] {
        let db = try openReadOnly()
        defer { sqlite3_close(db) }
        let statement = try prepare(db, sql: sql)
        defer { sqlite3_finalize(statement) }
        for (index, binding) in bindings.enumerated() {
            binding.apply(to: statement, index: Int32(index + 1))
        }

        var rows: [CatalogProduct] = []
        while sqlite3_step(statement) == SQLITE_ROW {
            let provider = Provider(rawValue: columnString(statement, 1)) ?? .frisco
            let currency = columnString(statement, 6, fallback: "PLN")
            rows.append(CatalogProduct(
                id: sqlite3_column_int64(statement, 0),
                provider: provider,
                externalID: columnString(statement, 2),
                name: columnString(statement, 3),
                brand: optionalColumnString(statement, 4),
                price: sqlite3_column_type(statement, 5) == SQLITE_NULL ? nil : moneyFromMinor(sqlite3_column_int64(statement, 5), currency: currency),
                measureText: optionalColumnString(statement, 7),
                available: sqlite3_column_type(statement, 8) == SQLITE_NULL ? nil : sqlite3_column_int(statement, 8) != 0,
                lastSeenAt: columnString(statement, 9)
            ))
        }
        return rows
    }

    private func openReadOnly() throws -> OpaquePointer? {
        guard FileManager.default.fileExists(atPath: databaseURL.path) else {
            throw MartMartError.commandFailed("Brak bazy katalogu: \(databaseURL.path)")
        }
        var db: OpaquePointer?
        let rc = sqlite3_open_v2(databaseURL.path, &db, SQLITE_OPEN_READONLY | SQLITE_OPEN_NOMUTEX, nil)
        if rc != SQLITE_OK {
            let message = db.map { String(cString: sqlite3_errmsg($0)) } ?? "sqlite open failed"
            if let db { sqlite3_close(db) }
            throw MartMartError.commandFailed(message)
        }
        return db
    }

    private func prepare(_ db: OpaquePointer?, sql: String) throws -> OpaquePointer? {
        var statement: OpaquePointer?
        let rc = sqlite3_prepare_v2(db, sql, -1, &statement, nil)
        if rc != SQLITE_OK {
            let message = db.map { String(cString: sqlite3_errmsg($0)) } ?? "sqlite prepare failed"
            throw MartMartError.commandFailed(message)
        }
        return statement
    }

    private func moneyFromMinor(_ minor: Int64, currency: String = "PLN") -> Money {
        Money(amount: Decimal(minor) / Decimal(100), currency: currency)
    }

    private func columnString(_ statement: OpaquePointer?, _ index: Int32, fallback: String = "") -> String {
        optionalColumnString(statement, index) ?? fallback
    }

    private func optionalColumnString(_ statement: OpaquePointer?, _ index: Int32) -> String? {
        guard sqlite3_column_type(statement, index) != SQLITE_NULL,
              let cString = sqlite3_column_text(statement, index) else { return nil }
        return String(cString: cString)
    }
}

enum SQLiteBinding {
    case int(Int)
    case text(String)

    func apply(to statement: OpaquePointer?, index: Int32) {
        switch self {
        case .int(let value): sqlite3_bind_int(statement, index, Int32(value))
        case .text(let value): sqlite3_bind_text(statement, index, value, -1, SQLITE_TRANSIENT)
        }
    }
}

private let SQLITE_TRANSIENT = unsafeBitCast(-1, to: sqlite3_destructor_type.self)
