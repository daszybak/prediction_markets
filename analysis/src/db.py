import os
from sqlalchemy import create_engine
from sqlalchemy.engine import Engine
import pandas as pd


def get_engine() -> Engine:
    """Get SQLAlchemy engine for TimescaleDB."""
    url = os.environ.get(
        "DATABASE_URL",
        "postgresql://prediction:prediction@localhost:5432/prediction"
    )
    return create_engine(url)


def query(sql: str, params: dict | None = None) -> pd.DataFrame:
    """Execute query and return DataFrame."""
    engine = get_engine()
    return pd.read_sql(sql, engine, params=params)


def get_markets(source: str | None = None) -> pd.DataFrame:
    """Get all markets, optionally filtered by source."""
    sql = "SELECT * FROM markets"
    if source:
        sql += " WHERE source = %(source)s"
    return query(sql, {"source": source} if source else None)


def get_prices(
    market_id: str,
    start: str | None = None,
    end: str | None = None
) -> pd.DataFrame:
    """Get price history for a market."""
    sql = """
        SELECT time, outcome, price, volume, bid, ask
        FROM prices
        WHERE market_id = %(market_id)s
    """
    params = {"market_id": market_id}

    if start:
        sql += " AND time >= %(start)s"
        params["start"] = start
    if end:
        sql += " AND time <= %(end)s"
        params["end"] = end

    sql += " ORDER BY time"
    return query(sql, params)
