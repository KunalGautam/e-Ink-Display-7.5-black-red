#!/usr/bin/env bash

# Exit immediately if any command fails
set -e

# Make sure the script is run with sudo/root privileges (since it modifies systemd)
if [[ $EUID -ne 0 ]]; then
   echo "Error: This script must be run as root (using sudo). Please run: sudo ./setup.sh install" 
   exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SERVICE_NAME="epaper-client"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

show_usage() {
    echo "Usage: sudo ./setup.sh [install | uninstall]"
    echo "  install   : Sets up the client dependencies and installs the systemd service"
    echo "  uninstall : Stops, disables, and removes the systemd service"
}

install_service() {
    echo "=== Starting e-Paper Client Installation ==="
    
    # 1. Check for Python & Pip
    if ! command -v python3 &>/dev/null; then
        echo "Error: Python 3 is not installed. Please install Python 3 first."
        exit 1
    fi
    
    if ! command -v pip3 &>/dev/null; then
        echo "Error: pip3 is not installed. Please install python3-pip first."
        exit 1
    fi

    # 2. Install dependencies
    echo "Installing Python dependencies from requirements.txt..."
    python3 -m pip install -r "${SCRIPT_DIR}/requirements.txt" || {
        echo "Warning: pip install failed. Attempting with --break-system-packages (for newer Debian/Raspberry Pi OS versions)..."
        python3 -m pip install -r "${SCRIPT_DIR}/requirements.txt" --break-system-packages
    }

    # 3. Prompt for configuration or use default
    echo ""
    echo "--- Configure Service Parameters ---"
    
    read -rp "Enter Go server URL [http://localhost:8080]: " SERVER_URL
    SERVER_URL=${SERVER_URL:-"http://localhost:8080"}
    
    read -rp "Enter refresh interval in seconds [300]: " REFRESH_INTERVAL_SECS
    REFRESH_INTERVAL_SECS=${REFRESH_INTERVAL_SECS:-"300"}
    
    read -rp "Enter Waveshare display driver name [epd7in5b_V2]: " EPD_DRIVER
    EPD_DRIVER=${EPD_DRIVER:-"epd7in5b_V2"}
    
    read -rp "Enable Hardware mock mode? (true/false) [false]: " MOCK_MODE
    MOCK_MODE=${MOCK_MODE:-"false"}

    # Detect current non-root user (who ran sudo) for the service execution
    RUN_USER=${SUDO_USER:-"pi"}
    if [ "$RUN_USER" = "root" ]; then
        # Default fallback if run directly as root
        RUN_USER="pi"
    fi
    
    echo "Configuring service to run as user: ${RUN_USER}"

    # 4. Generate systemd service file
    echo "Generating ${SERVICE_FILE}..."
    cat <<EOF > "${SERVICE_FILE}"
[Unit]
Description=e-Paper E-Ink Display Client Service
After=network.target

[Service]
Type=simple
User=${RUN_USER}
WorkingDirectory=${SCRIPT_DIR}
Environment=SERVER_URL=${SERVER_URL}
Environment=REFRESH_INTERVAL_SECS=${REFRESH_INTERVAL_SECS}
Environment=EPD_DRIVER=${EPD_DRIVER}
Environment=MOCK_MODE=${MOCK_MODE}
ExecStart=/usr/bin/python3 client.py
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    # 5. Load and start systemd service
    echo "Reloading systemd daemon..."
    systemctl daemon-reload
    
    echo "Enabling ${SERVICE_NAME} on boot..."
    systemctl enable "${SERVICE_NAME}"
    
    echo "Starting ${SERVICE_NAME} service..."
    systemctl start "${SERVICE_NAME}"
    
    echo ""
    echo "=== Installation Completed Successfully! ==="
    echo "To check the service status, run:"
    echo "  sudo systemctl status ${SERVICE_NAME}"
    echo "To view live logs, run:"
    echo "  sudo journalctl -u ${SERVICE_NAME} -f"
}

uninstall_service() {
    echo "=== Starting e-Paper Client Uninstallation ==="
    
    if [ -f "${SERVICE_FILE}" ]; then
        echo "Stopping ${SERVICE_NAME} service..."
        systemctl stop "${SERVICE_NAME}" || true
        
        echo "Disabling ${SERVICE_NAME} service..."
        systemctl disable "${SERVICE_NAME}" || true
        
        echo "Removing systemd service file..."
        rm -f "${SERVICE_FILE}"
        
        echo "Reloading systemd daemon..."
        systemctl daemon-reload
        
        echo "Systemd service removed successfully."
    else
        echo "Service file ${SERVICE_FILE} does not exist. Skipping service removal."
    fi
    
    # Optionally remove local state file
    STATE_FILE="${SCRIPT_DIR}/.client_state.json"
    if [ -f "${STATE_FILE}" ]; then
        read -rp "Do you want to delete the cached client state file (.client_state.json)? (y/n) [n]: " REMOVE_STATE
        if [[ $REMOVE_STATE =~ ^[Yy]$ ]]; then
            rm -f "${STATE_FILE}"
            echo "State cache file removed."
        fi
    fi
    
    echo ""
    echo "=== Uninstallation Completed Successfully! ==="
}

# Main Execution flow
if [[ $# -ne 1 ]]; then
    show_usage
    exit 1
fi

ACTION="$1"
case "$ACTION" in
    install)
        install_service
        ;;
    uninstall)
        uninstall_service
        ;;
    *)
        echo "Error: Unknown action '${ACTION}'"
        show_usage
        exit 1
        ;;
esac
