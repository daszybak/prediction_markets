-- Drop continuous aggregates (policies are dropped automatically)
DROP MATERIALIZED VIEW IF EXISTS order_book_snapshots_1d;
DROP MATERIALIZED VIEW IF EXISTS order_book_snapshots_1h;
DROP MATERIALIZED VIEW IF EXISTS order_book_snapshots_5m;
DROP MATERIALIZED VIEW IF EXISTS order_book_snapshots_1m;
