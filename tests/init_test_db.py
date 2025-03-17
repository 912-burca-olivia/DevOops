import os
import sqlite3
from contextlib import closing

DATABASE = os.getenv("DATABASE", "/app/test_minitwit.db")

def reset_test_db():
    """Ensures the test database is reset before each test session."""
    db_dir = os.path.dirname(DATABASE)
    if db_dir and not os.path.exists(db_dir):
        os.makedirs(db_dir, exist_ok=True)

    # Remove the existing test database
    if os.path.isfile(DATABASE):
        os.remove(DATABASE)

    # Initialize schema and return a database connection
    db_client = sqlite3.connect(DATABASE, timeout=5)
    with closing(db_client.cursor()) as cursor:
        with open("/tests/schema.sql") as fp:
            cursor.executescript(fp.read())
        db_client.commit()

    print("✅ Test database initialized.")
    return db_client