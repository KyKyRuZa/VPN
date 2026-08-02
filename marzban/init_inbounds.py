#!/usr/bin/env python3
"""Initialize Marzban inbounds in PostgreSQL database."""

import os
import sys

import psycopg2

DB_HOST = os.environ.get("DB_HOST", "postgres")
DB_PORT = os.environ.get("DB_PORT", "5432")
DB_USER = os.environ.get("POSTGRES_USER", "vpn_user")
DB_PASSWORD = os.environ.get("POSTGRES_PASSWORD", "")
DB_NAME = os.environ.get("POSTGRES_DB", "marzban_db")

INBOUND_TAGS = ["VLESS Reality", "Trojan TLS"]


def main() -> None:
    print(f"Connecting to {DB_USER}@{DB_HOST}:{DB_PORT}/{DB_NAME}")
    conn = psycopg2.connect(
        host=DB_HOST,
        port=DB_PORT,
        user=DB_USER,
        password=DB_PASSWORD,
        dbname=DB_NAME,
    )
    conn.autocommit = True
    cur = conn.cursor()

    for tag in INBOUND_TAGS:
        cur.execute(
            "INSERT INTO inbounds (tag) VALUES (%s) ON CONFLICT (tag) DO NOTHING",
            (tag,),
        )
        print(f"Upserted inbound: {tag}")

    cur.close()
    conn.close()
    print("Done")


if __name__ == "__main__":
    main()
