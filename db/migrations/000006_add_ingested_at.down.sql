-- Remove ingested_at columns
ALTER TABLE order_book_snapshots DROP COLUMN ingested_at;
ALTER TABLE trades DROP COLUMN ingested_at;
ALTER TABLE order_book_metrics DROP COLUMN ingested_at;
