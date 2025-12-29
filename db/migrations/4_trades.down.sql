SELECT remove_retention_policy('trades', if_exists => true);
SELECT remove_compression_policy('trades', if_exists => true);
DROP TABLE IF EXISTS trades;
