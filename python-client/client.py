#!/usr/bin/env python3
import os
import sys
import time
import json
import logging
import hashlib
from io import BytesIO
import requests
from PIL import Image

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

# Configuration parameters (loaded from env variables or defaults)
SERVER_URL = os.getenv("SERVER_URL", "http://localhost:8080").rstrip('/')
REFRESH_INTERVAL = int(os.getenv("REFRESH_INTERVAL_SECS", "300"))  # 5 minutes
DISPLAY_MODE = os.getenv("DISPLAY_MODE", "png").lower()            # "png" or "raw"
EPD_DRIVER = os.getenv("EPD_DRIVER", "epd7in5b_V2")                # Waveshare 7.5" V2 B/C-type
MOCK_MODE = os.getenv("MOCK_MODE", "false").lower() in ("true", "1", "yes")

# Validate display mode
if DISPLAY_MODE not in ("png", "raw"):
    logger.error("DISPLAY_MODE must be either 'png' or 'raw'. Defaulting to 'png'.")
    DISPLAY_MODE = "png"

# Setup state file path to track last rendered image hash and timestamp
STATE_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".client_state.json")

# Setup e-ink driver reference
epd = None
if not MOCK_MODE:
    try:
        import importlib
        logger.info(f"Loading driver waveshare_epd.{EPD_DRIVER}...")
        epd_module = importlib.import_module(f"waveshare_epd.{EPD_DRIVER}")
        epd = epd_module.EPD()
        logger.info("Driver loaded successfully.")
    except ImportError as e:
        logger.error(
            f"Failed to import waveshare_epd.{EPD_DRIVER}. Make sure waveshare-epaper library is installed, "
            f"or run in mock mode by setting MOCK_MODE=true. Error: {e}"
        )
        sys.exit(1)
else:
    logger.info("Running in DEVELOPER MOCK MODE. Output channels will be saved to disk.")


def load_state():
    """Loads client state containing the last updated image hash and timestamp."""
    state = {"last_hash": "", "last_update": 0.0}
    if os.path.exists(STATE_FILE):
        try:
            with open(STATE_FILE, "r") as f:
                state = json.load(f)
            logger.debug(f"Loaded client state: {state}")
        except Exception as e:
            logger.warning(f"Failed to read client state file: {e}")
    return state


def save_state(state):
    """Saves client state containing the last updated image hash and timestamp."""
    try:
        with open(STATE_FILE, "w") as f:
            json.dump(state, f)
        logger.debug(f"Saved client state: {state}")
    except Exception as e:
        logger.warning(f"Failed to save client state file: {e}")


def split_channels(img_rgb: Image.Image):
    """
    Takes an RGB Image, processes its pixels, and splits it into
    black/white and red/white monochrome '1' images.
    Using getdata() and putdata() is highly optimized in Pillow.
    """
    logger.info("Processing PNG layout and separating colors...")
    start_time = time.time()
    
    img_data = img_rgb.getdata()
    black_pixels = []
    red_pixels = []

    for r, g, b in img_data:
        # Check for red (high red, low green and blue)
        if r > 130 and g < 120 and b < 120:
            black_pixels.append(255) # white
            red_pixels.append(0)     # red/active
        # Check for black/grey (any dark or grey pixel is treated as black to capture anti-aliasing edges)
        elif r < 140 and g < 140 and b < 140:
            black_pixels.append(0)   # black/active
            red_pixels.append(255) # white
        else:
            black_pixels.append(255) # white
            red_pixels.append(255) # white

    black_img = Image.new('1', img_rgb.size)
    black_img.putdata(black_pixels)

    red_img = Image.new('1', img_rgb.size)
    red_img.putdata(red_pixels)
    
    elapsed = time.time() - start_time
    logger.info(f"Color separation finished in {elapsed:.3f} seconds.")
    return black_img, red_img


def fetch_image_with_retry():
    """
    Fetches the layout image from the Go server with exponential backoff on failure.
    """
    endpoint = f"{SERVER_URL}/image" if DISPLAY_MODE == "png" else f"{SERVER_URL}/image/raw"
    backoff = 5
    max_backoff = 60

    while True:
        try:
            logger.info(f"Fetching layout from server: {endpoint}")
            response = requests.get(endpoint, timeout=15)
            response.raise_for_status()
            return response.content
        except requests.RequestException as e:
            logger.warning(f"Connection error: {e}. Retrying in {backoff} seconds...")
            time.sleep(backoff)
            backoff = min(backoff * 2, max_backoff)


def update_display():
    """
    Fetches layout, computes hash, verifies if changes exist, and updates display.
    """
    data = fetch_image_with_retry()
    
    # 1. Compute hash of the received image bytes
    current_hash = hashlib.sha256(data).hexdigest()
    
    # 2. Check last update state
    state = load_state()
    last_hash = state.get("last_hash", "")
    last_update = state.get("last_update", 0.0)
    
    now = time.time()
    elapsed_since_update = now - last_update
    
    # e-paper guidelines recommend a refresh at least once every 24 hours to prevent burn-in/ghosting
    force_refresh_threshold = 24 * 3600.0 # 24 hours
    force_refresh = elapsed_since_update >= force_refresh_threshold
    
    if current_hash == last_hash and not force_refresh:
        logger.info(
            f"Display content unchanged (SHA256: {current_hash[:10]}...). "
            f"Last refresh was {elapsed_since_update / 60:.1f} minutes ago. Skipping update."
        )
        return
        
    if current_hash != last_hash:
        logger.info(f"New layout content detected (SHA256: {current_hash[:10]}...). Proceeding with display update.")
    elif force_refresh:
        logger.info("Anti-ghosting safety refresh triggered (24 hours elapsed). Proceeding with display update.")

    # 3. Process and write to display
    update_success = False
    
    if DISPLAY_MODE == "png":
        try:
            img = Image.open(BytesIO(data)).convert('RGB')
        except Exception as e:
            logger.error(f"Failed to parse PNG response: {e}")
            return
        
        black_img, red_img = split_channels(img)
        
        if MOCK_MODE:
            black_img.save("mock_black_channel.png")
            red_img.save("mock_red_channel.png")
            logger.info("Mock Mode: Saved mock_black_channel.png and mock_red_channel.png")
            update_success = True
        else:
            try:
                logger.info("Initializing e-paper panel...")
                epd.init()
                logger.info("Sending image buffers to display panel (Full Refresh)...")
                epd.display(epd.getbuffer(black_img), epd.getbuffer(red_img))
                logger.info("Display update complete. Putting display to sleep.")
                epd.sleep()
                update_success = True
            except Exception as e:
                logger.error(f"Error driving physical e-ink display: {e}")
    
    else:  # raw mode
        expected_len = 96000
        if len(data) != expected_len:
            logger.error(f"Received raw buffer of incorrect length: {len(data)} (expected {expected_len})")
            return
        
        black_bytes = data[:48000]
        red_bytes = data[48000:]
        
        if MOCK_MODE:
            logger.info("Reconstructing mock channel previews from raw bytes...")
            width, height = 800, 480
            black_img = Image.frombytes('1', (width, height), black_bytes)
            red_img = Image.frombytes('1', (width, height), red_bytes)
            black_img.save("mock_black_channel.png")
            red_img.save("mock_red_channel.png")
            logger.info("Mock Mode: Saved mock_black_channel.png and mock_red_channel.png")
            update_success = True
        else:
            try:
                logger.info("Initializing e-paper panel...")
                epd.init()
                logger.info("Sending raw byte buffers to display panel...")
                epd.display(list(black_bytes), list(red_bytes))
                logger.info("Display update complete. Putting display to sleep.")
                epd.sleep()
                update_success = True
            except Exception as e:
                logger.error(f"Error driving physical e-ink display with raw bytes: {e}")

    # 4. Save state if the hardware update succeeded
    if update_success:
        state["last_hash"] = current_hash
        state["last_update"] = now
        save_state(state)


def main():
    logger.info("Starting e-ink display client loop...")
    logger.info(f"Settings - Server: {SERVER_URL}, Mode: {DISPLAY_MODE}, Interval: {REFRESH_INTERVAL}s")
    
    while True:
        try:
            update_display()
        except KeyboardInterrupt:
            logger.info("Client terminated by user.")
            break
        except Exception as e:
            logger.critical(f"Unexpected error in client loop: {e}", exc_info=True)
            
        logger.info(f"Sleeping for {REFRESH_INTERVAL} seconds until next update...")
        time.sleep(REFRESH_INTERVAL)


if __name__ == "__main__":
    main()
