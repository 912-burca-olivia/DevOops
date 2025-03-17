from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.common.keys import Keys
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.firefox.service import Service
from selenium.webdriver.firefox.options import Options

GUI_URL = "http://app_test:8080/register"

def _register_user_via_gui(driver, data):
    driver.get(GUI_URL)

    wait = WebDriverWait(driver, 5)
    buttons = wait.until(EC.presence_of_all_elements_located((By.CLASS_NAME, "actions")))
    input_fields = driver.find_elements(By.TAG_NAME, "input")

    for idx, str_content in enumerate(data):
        input_fields[idx].send_keys(str_content)
    input_fields[4].send_keys(Keys.RETURN)

    wait = WebDriverWait(driver, 5)
    flashes = wait.until(EC.presence_of_all_elements_located((By.CLASS_NAME, "flashes")))

    return flashes

def test_register_user_via_gui_and_check_db_entry(test_db):
    """E2E test: Register user via UI and verify database entry."""
    firefox_options = Options()
    firefox_options.add_argument("--headless")

    # STILL HAS TO BE CHANGED TO HANDLE GORM ACCESS

    with webdriver.Firefox(service=Service("/usr/local/bin/geckodriver"), options=firefox_options) as driver:
        cursor = test_db.cursor()  # Now test_db is a valid connection

        cursor.execute("SELECT username FROM users WHERE username = ?", ("Me",))
        assert cursor.fetchone() is None  # Ensure user doesn't exist

        generated_msg = _register_user_via_gui(driver, ["Me", "me@some.where", "secure123", "secure123"])[0].text
        expected_msg = "You were successfully registered and can login now"
        assert generated_msg == expected_msg

        cursor.execute("SELECT username FROM users WHERE username = ?", ("Me",))
        assert cursor.fetchone()[0] == "Me"