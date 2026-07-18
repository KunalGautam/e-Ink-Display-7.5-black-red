# Multi-Canvas Widget-Based E-Ink Platform

A high-performance, containerized Go server and Python client system for rendering and displaying customizable widget-based dashboards on any e-Paper panel (supporting Waveshare 7.5", 4.2", 2.9" and other mono/3-color screens).

Rendering is performed entirely on the Go backend using pure vector drawing libraries (`fogleman/gg`), offering lightning-fast layout composition, automatic font loading/caching from Google Fonts, and dynamic MQTT topic bindings per widget.

---

## 1. API Documentation

### Canvas Management (CRUD)
*   **`POST /api/canvas`**: Create or overwrite a canvas profile.
    *   *Payload:*
        ```json
        {
          "id": "living_room",
          "device_type": "waveshare_7in5_v2",
          "timezone": "Asia/Kolkata"
        }
        ```
        *(Pre-configured types: `waveshare_7in5_v2` (800x480 BWR), `waveshare_7in5_mono`, `waveshare_4in2` (400x300 mono), `waveshare_2in9_bwr`, `waveshare_2in9_mono`)*
    *   *Custom Dims Payload:*
        ```json
        {
          "id": "custom_screen",
          "device_type": "custom",
          "width": 640,
          "height": 384,
          "color_mode": "mono",
          "timezone": "America/New_York"
        }
        ```
*   **`GET /api/canvas`**: List all saved canvas profiles.
*   **`DELETE /api/canvas/{id}`**: Delete a canvas and its widgets.

### Widget Configuration
*   **`POST /api/widget`**: Add or update a widget inside a canvas profile.
    *   *Payload:*
        ```json
        {
          "id": "clock_widget",
          "canvas_id": "living_room",
          "type": "datetime",
          "x": 20, "y": 20, "width": 250, "height": 80,
          "color_fg": "#FF0000", "color_bg": "",
          "font_url": "https://fonts.googleapis.com/css2?family=Outfit:wght@600",
          "font_size": 24,
          "custom_config": "{\"format\": \"15:04\"}"
        }
        ```
*   **`GET /api/canvas/{id}`**: List all widgets registered to a canvas.
*   **`DELETE /api/widget/{id}`**: Remove a widget.

### Display Render Endpoints
*   **`GET /canvas/{id}/preview`**: Returns a standard diagnostic PNG preview image (useful for web layout adjustments).
*   **`GET /canvas/{id}/render`**: Returns raw packed active-low display bytes. Mono payloads are `(W * H) / 8` bytes. BWR payloads are doubled, concatenating the black buffer first and the red buffer second.

---

## 2. Docker Server Deployment

The Go server can be spun up along with an Eclipse Mosquitto MQTT broker using Docker Compose:

1.  **Clone and Enter Repository**:
    ```bash
    git clone https://github.com/KunalGautam/e-Ink-Display-7.5-black-red.git
    cd e-Ink-Display-7.5-black-red/go-server
    ```
2.  **Initialize Environment Ports**:
    ```bash
    cp .env.example .env
    ```
3.  **Start Services**:
    ```bash
    docker-compose up -d --build
    ```
4.  **Admin web**: Access `http://localhost:8080/settings` in your browser. (Default credentials: `admin` / `admin`).

---

## 3. Python Client Installation (Raspberry Pi)

Each physical Raspberry Pi runs a lightweight client loop pointing to its target canvas profile:

1.  **Run Setup Installer**:
    ```bash
    cd python-client
    sudo chmod +x setup.sh
    sudo ./setup.sh install
    ```
2.  **Provide Configurations**:
    The installer will prompt you for the `Canvas ID`, `Go Server URL`, `Waveshare Driver Model`, and `Poll Interval`. This writes a local config file `client_config.json` and registers the client loop as an autostarting systemd service:
    ```bash
    sudo systemctl status epaper-client.service
    ```
