#!/usr/bin/env python3
import os
import sys
import time
import json
import logging
import hashlib
import requests

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

# Path to config file and state file
CONFIG_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "client_config.json")
STATE_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".client_state.json")

# Default configurations
config = {
    "canvas_id": "default",
    "backend_url": "http://localhost:8080",
    "display_driver": "epd7in5b_V2",
    "poll_interval": 300,
    "mock_mode": False
}

# Load configurations from client_config.json if exists
if os.path.exists(CONFIG_FILE):
    try:
        with open(CONFIG_FILE, "r") as f:
            user_config = json.load(f)
            config.update(user_config)
        logger.info(f"Loaded configuration from {CONFIG_FILE}")
    except Exception as e:
        logger.warning(f"Failed to parse config file: {e}. Using defaults/environment.")

# Fallback/override with environment variables
CANVAS_ID = os.getenv("CANVAS_ID", config["canvas_id"])
BACKEND_URL = os.getenv("BACKEND_URL", config["backend_url"]).rstrip('/')
EPD_DRIVER = os.getenv("EPD_DRIVER", config["display_driver"])
POLL_INTERVAL = int(os.getenv("POLL_INTERVAL", str(config["poll_interval"])))
MOCK_MODE = os.getenv("MOCK_MODE", "true" if config["mock_mode"] else "false").lower() in ("true", "1", "yes")

# Setup e-ink driver reference
epd = None
is_bwr = "b" in EPD_DRIVER.lower()  # standard Waveshare driver naming: 'b' suffix denotes black/white/red

if not MOCK_MODE:
    try:
        import importlib
        logger.info(f"Loading driver waveshare_epd.{EPD_DRIVER}...")
        epd_module = importlib.import_module(f"waveshare_epd.{EPD_DRIVER}")
        epd = epd_module.EPD()
        logger.info("Driver loaded successfully.")
    except ImportError as e:
        logger.error(
            f"Failed to import waveshare_epd.{EPD_DRIVER}. Install waveshare-epaper, or set mock_mode to true. Error: {e}"
        )
        sys.exit(1)
else:
    logger.info("Running in DEVELOPER MOCK MODE. Bytes will be parsed to mock images on disk.")


def load_state():
    state = {"last_hash": "", "last_update": 0.0}
    if os.path.exists(STATE_FILE):
        try:
            with open(STATE_FILE, "r") as f:
                state = json.load(f)
        except Exception as e:
            logger.warning(f"Failed to read state cache: {e}")
    return state


def save_state(state):
    try:
        with open(STATE_FILE, "w") as f:
            json.dump(state, f)
    except Exception as e:
        logger.warning(f"Failed to save state cache: {e}")


def fetch_packed_render_bytes():
    endpoint = f"{BACKEND_URL}/canvas/{CANVAS_ID}/render"
    backoff = 5
    max_backoff = 60

    while True:
        try:
            logger.info(f"Fetching packed render stream from: {endpoint}")
            response = requests.get(endpoint, timeout=20)
            response.raise_for_status()
            return response.content
        except requests.RequestException as e:
            logger.warning(f"Connection error: {e}. Retrying in {backoff} seconds...")
            time.sleep(backoff)
            backoff = min(backoff * 2, max_backoff)


def update_display():
    data = fetch_packed_render_bytes()
    current_hash = hashlib.sha256(data).hexdigest()

    state = load_state()
    last_hash = state.get("last_hash", "")
    last_update = state.get("last_update", 0.0)

    now = time.time()
    elapsed = now - last_update
    force_refresh = elapsed >= (24 * 3600.0) # 24 hours anti-ghosting threshold

    if current_hash == last_hash and not force_refresh:
        logger.info(f"Display content unchanged (SHA256: {current_hash[:10]}). Skipping screen refresh.")
        return

    logger.info("New content or safety refresh triggered. Initiating screen update...")
    update_success = False

    # Extract buffers based on BWR or Monochrome configurations
    if is_bwr:
        half_len = len(data) // 2
        black_bytes = data[:half_len]
        red_bytes = data[half_len:]
    else:
        black_bytes = data
        red_bytes = None

    if MOCK_MODE:
        try:
            from PIL import Image
            # Reconstruct mock preview images for developer diagnostics
            width = epd.width if epd else 800
            height = epd.height if epd else 480
            black_img = Image.frombytes('1', (width, height), bytes(black_bytes))
            black_img.save("mock_black_channel.png")
            if red_bytes:
                red_img = Image.frombytes('1', (width, height), bytes(red_bytes))
                red_img.save("mock_red_channel.png")
            logger.info("Mock Mode: Saved visual output channels to disk.")
            update_success = True
        except Exception as e:
            logger.error(f"Failed to generate mock image previews: {e}")
    else:
        try:
            logger.info("Initializing e-paper...")
            epd.init()
            logger.info("Writing buffer bytes to screen...")
            if is_bwr and red_bytes:
                epd.display(list(black_bytes), list(red_bytes))
            else:
                epd.display(list(black_bytes))
            logger.info("Update finished. Sleeping panel.")
            epd.sleep()
            update_success = True
        except Exception as e:
            logger.error(f"SPI hardware write error: {e}")

    if update_success:
        state["last_hash"] = current_hash
        state["last_update"] = now
        save_state(state)


def main():
    logger.info("E-Ink Display Client Daemon Started.")
    logger.info(f"Config - Canvas ID: {CANVAS_ID}, Backend: {BACKEND_URL}, Interval: {POLL_INTERVAL}s")
    
    while True:
        try:
            update_display()
        except KeyboardInterrupt:
            logger.info("Terminated by user signal.")
            break
        except Exception as e:
            logger.error(f"Loop runtime exception: {e}", exc_info=True)
            
        time.sleep(POLL_INTERVAL)


if __name__ == "__main__":
    main()
