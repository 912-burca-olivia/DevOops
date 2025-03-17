import pytest
from init_test_db import reset_test_db

@pytest.fixture(scope="function", autouse=True)
def test_db():
    """Resets the database before each test"""
    db_client = reset_test_db()  # reset and initialize a new DB
    yield db_client
    db_client.close()  # DB connection closes after test