# Waveshare 7.5" 3-Color E-Ink Display System

A Go API server and Python client system for rendering and displaying a high-resolution, 3-color (Black/White/Red) dashboard on a Waveshare 7.5" e-ink panel (800x480px). Fully supports Devanagari (Hindi/Marathi) and English scripts using the **Poppins** font family with pixel-accurate text wrapping.

## System Overview

The system consists of:
1. **Go API Server**: Connects to an MQTT broker, fetches and parses today's events from a Google Calendar private iCal feed, caches notes, emails, and schedule events, and serves them. It delegates the 2D layout composition to a Python script using Pillow's native shaping engine for perfect Devanagari script support.
2. **Python Client**: Runs on the Raspberry Pi connected to the display, polls the Go server, separates colors, drives the screen using the official Waveshare library, and puts the panel into sleep mode between refreshes.

---

## Hardware Wiring Guide (Raspberry Pi to Waveshare 7.5" E-Ink)

Connect the Waveshare HAT/module to the Raspberry Pi GPIO headers according to the following standard SPI layout:

| E-Ink Pin | Cable Color | Raspberry Pi Pin | Function |
| :--- | :--- | :--- | :--- |
| **VCC** | Red | Pin 1 or 17 (3.3V) | Power |
| **GND** | Black | Pin 9, 14, 20 or 25 (GND) | Ground |
| **DIN (MOSI)** | Blue | Pin 19 (GPIO 10 / SPI0 MOSI) | SPI Data |
| **CLK** | Yellow | Pin 23 (GPIO 11 / SPI0 SCLK) | SPI Clock |
| **CS** | Orange | Pin 24 (GPIO 8 / SPI0 CE0) | SPI Chip Select |
| **DC** | Green | Pin 22 (GPIO 25) | Data / Command Select |
| **RST** | White | Pin 11 (GPIO 17) | Hardware Reset |
| **BUSY** | Purple | Pin 18 (GPIO 24) | Busy Signal |

Make sure **SPI is enabled** on your Raspberry Pi:
```bash
sudo raspi-config
# Navigate to: Interface Options -> SPI -> Enable -> Yes -> Finish
```

---

## MQTT Topics & Payload Schemas

The Go server subscribes to these topics and caches the latest retained messages in-memory.

### 1. Topic: `home/eink/emails`
Expects a JSON array of email objects. Shows sender and subject.
```json
[
  {"sender": "Alice Cooper <alice@example.com>", "subject": "Project update: layout engine"},
  {"sender": "अमित शर्मा <amit@example.com>", "subject": "मीटिंग का समय बदलाव"}
]
```

### 2. Topic: `home/eink/notes`
Expects a JSON array of strings, displaying each as a bulleted note. Supports Hindi.
```json
[
  "सब्जियां और फल बाजार से खरीदना न भूलें",
  "Review the implementation plan for epaper display system and sign off",
  "Water the backyard plants at 6:00 PM today"
]
```

### 3. Topic: `home/eink/calendar`
Expects a JSON array of daily event objects. Displayed in the "SCHEDULE" section on the left.
```json
[
  {"title": "Doctor Appointment", "time": "09:00 - 10:00"},
  {"title": "अमित के साथ बैठक", "time": "14:00 - 15:00"},
  {"title": "सब्जी मंडी जाना", "time": "All Day"}
]
```

### 4. Topic: `home/eink/weather`
Expects a JSON object detailing current weather observations and forecast increments. Highly recommended for weather layouts:
```json
{
  "temp": 23.5,
  "condition": "Partly Cloudy",
  "humidity": 65,
  "pressure": 1011.2,
  "wind_speed": 4.8,
  "forecast": [
    {"time": "10 AM", "temp": 22.0, "condition": "Sunny"},
    {"time": "1 PM", "temp": 24.5, "condition": "Partly Cloudy"},
    {"time": "4 PM", "temp": 23.0, "condition": "Cloudy"},
    {"time": "7 PM", "temp": 21.0, "condition": "Rainy"}
  ]
}
```

---

## Go API Server Setup

### Requirements & System Setup

The Go server uses a Python helper script to render layouts. This requires Python 3.9+ and Pillow. For complex Devanagari text shaping to work, you must install the `libraqm` library on the server host machine.

Select the installation command for your Linux distribution:

*   **Debian / Ubuntu / Raspberry Pi OS**:
    ```bash
    sudo apt-get update
    sudo apt-get install -y python3-pip python3-pil python3-venv libraqm-dev
    ```
*   **Arch Linux**:
    ```bash
    sudo pacman -Syu python-pip python-pillow python-virtualenv raqm
    ```
*   **Fedora / RHEL**:
    ```bash
    sudo dnf install -y python3-pip python3-pillow python3-virtualenv libraqm-devel
    ```
*   **macOS**:
    ```bash
    brew install raqm
    ```

### Installation & Run

1.  **Navigate to the server folder**:
    ```bash
    cd go-server
    ```

2.  **Initialize the Python Virtual Environment**:
    To comply with PEP 668 on modern Linux distributions, create an isolated virtual environment inside the `go-server` directory and install Pillow:
    ```bash
    python3 -m venv venv
    venv/bin/pip install pillow
    ```
    *(Note: The Go server will automatically discover this virtual environment and use `venv/bin/python` for image rendering.)*

3.  **Tidy Go module dependencies**:
    ```bash
    go mod tidy
    ```

4.  **Configure settings**:
    Copy the example configuration file and adjust your broker, topics, timezone, and calendar settings:
    ```bash
    cp config.yaml.example config.yaml
    ```

5.  **Build and run the server**:
    ```bash
    go build -o eink-server main.go
    ./eink-server -config config.yaml
    ```

*(Note: On initial startup, the Go server automatically checks for and downloads Poppins-Regular and Poppins-Bold fonts from Google Fonts into `assets/fonts/` if they are not already present.)*

---

## Web Configuration Portal

The Go server serves a Basic Auth-protected web administration page to configure parameters dynamically.

*   **URL**: `http://<server-ip>:8080/settings`
*   **Default Credentials**:
    *   **Username**: `admin`
    *   **Password**: `admin`

### Key Features
*   **Dynamic Setting Form**: Modify MQTT broker details, credentials, and topics; adjust display dimensions; change timezone and server port.
*   **Live Layout Preview**: Instantly view what the dashboard will look like on the physical screen.
*   **Process Control**: Click "Restart Server" to gracefully shut down the Go process. Process managers (like `systemd` or `docker` with `restart: always`) will automatically restart it using the new settings.
*   **Persistent Offline Caches**: Note lists and emails are saved in an SQLite database file (`epaper.db`). If the MQTT broker or network goes offline, the server pre-populates dashboard caches from the database.

---

## Python Client Setup (Raspberry Pi)

### 1. Hardware Connections (SPI)

Ensure the SPI interface is enabled on your Raspberry Pi:
1. Run `sudo raspi-config`.
2. Navigate to **Interface Options** > **SPI** and select **Yes** to enable.
3. Reboot the Pi.

Connect the Waveshare 7.5" e-Paper display HAT to the Raspberry Pi GPIO headers according to the standard SPI wiring diagram:

| e-Paper HAT | Raspberry Pi GPIO | Description |
| :--- | :--- | :--- |
| **VCC** | 3.3V (Pin 1 or 17) | Power input |
| **GND** | GND (Pin 9 or 25) | Ground |
| **DIN** | MOSI (Pin 19 / GPIO 10) | SPI Master Out Slave In |
| **CLK** | SCLK (Pin 23 / GPIO 11) | SPI Clock |
| **CS** | Chip Select (Pin 24 / GPIO 8) | SPI Chip Selection (Active Low) |
| **D/C** | Command/Data (Pin 22 / GPIO 25)| Data/Command selection pin |
| **RST** | Reset (Pin 11 / GPIO 17) | Reset pin (Active Low) |
| **BUSY**| Busy Status (Pin 18 / GPIO 24)| Busy status output (Active Low) |

### 2. Client Dependencies

Install required Python dependencies in the `python-client` folder:
```bash
cd python-client
pip3 install -r requirements.txt
```
*(Note: If you are running a physical display, you must also install the Waveshare e-Paper library dependencies. Follow the official Waveshare guide or install `spidev` and `RPi.GPIO` packages).*

### 3. Execution Modes

You can execute the client in either developer mock mode or production hardware mode.

#### Developer Mock Mode (No Hardware Required)
Saves the separated black and red layout channels as PNG images locally for verification:
```bash
MOCK_MODE=true SERVER_URL="http://localhost:8080" python3 client.py
```
This produces `mock_black_channel.png` and `mock_red_channel.png` in the client directory.

#### Production Hardware Mode
Drives the physical Waveshare panel:
```bash
SERVER_URL="http://<go-server-ip>:8080" REFRESH_INTERVAL_SECS=300 EPD_DRIVER="epd7in5b_V2" python3 client.py
```

### 4. Running the Client as a Service (systemd)

A helper script [setup.sh](file:///home/kunal/Projects/epaper-display/python-client/setup.sh) is provided to easily install, configure, or uninstall the systemd service.

#### Installation & Configuration
Run the setup script with the `install` argument:
```bash
cd python-client
sudo ./setup.sh install
```
This script will:
* Check and install required Python dependencies.
* Prompt you for Go server configuration details (URL, refresh interval, display driver, and mock mode).
* Detect your user profile, generate the `/etc/systemd/system/epaper-client.service` file, reload systemd, and enable and start the service.

#### Uninstallation & Cleanup
To stop, disable, and clean up the service, run:
```bash
cd python-client
sudo ./setup.sh uninstall
```
This will safely stop the daemon, delete the systemd configuration file, reload systemd, and optionally prompt to clear the cached layout state file.

#### Service Management
You can also manage the service manually using standard systemctl commands:
* **Check Status**: `sudo systemctl status epaper-client.service`
* **Live Logs**: `sudo journalctl -u epaper-client.service -f`
* **Restart**: `sudo systemctl restart epaper-client.service`

### 5. Automatic Pull & Skip-Refresh Mechanics

The client is designed to operate autonomously and minimize e-paper wear:
*   **Continuous Loop**: The script runs an infinite loop, querying the server at the configured `REFRESH_INTERVAL_SECS` interval.
*   **SHA-256 Hash Matching**: Upon fetching the layout image, the client calculates its SHA-256 hash and compares it to the previous hash stored in `.client_state.json`.
*   **Refresh Skipping**: If the image hash is identical and the screen was refreshed within the last 24 hours, the client **skips** the update. This eliminates the high-wear 20-second screen flicker cycles and saves power during idle periods.
*   **Anti-Ghosting Safeguard**: If 24 hours pass without a refresh, the client forces a layout redraw even if the image remains unchanged, ensuring the e-paper panel stays healthy and free of ghosting.
