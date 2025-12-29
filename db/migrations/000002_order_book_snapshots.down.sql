SELECT remove_retention_policy('order_book_snapshots', if_exists => true);
SELECT remove_compression_policy('order_book_snapshots', if_exists => true);
DROP TABLE IF EXISTS order_book_snapshots;
