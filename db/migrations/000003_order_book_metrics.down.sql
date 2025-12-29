SELECT remove_retention_policy('order_book_metrics', if_exists => true);
SELECT remove_compression_policy('order_book_metrics', if_exists => true);
DROP TABLE IF EXISTS order_book_metrics;
