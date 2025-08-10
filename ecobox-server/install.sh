#!/bin/bash

# Network Dashboard Installation Script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
INSTALL_DIR="/opt/ecobox-server"
SERVICE_FILE="/etc/systemd/system/ecobox-server.service"
USER="dashboard"
GROUP="dashboard"

echo -e "${GREEN}Network Dashboard Installation Script${NC}"
echo "======================================"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root (use sudo)${NC}"
   exit 1
fi

# Create user and group
echo -e "${YELLOW}Creating user and group...${NC}"
if ! id "$USER" &>/dev/null; then
    useradd --system --shell /bin/false --home-dir "$INSTALL_DIR" --create-home "$USER"
    echo "Created user: $USER"
else
    echo "User $USER already exists"
fi

# Create installation directory
echo -e "${YELLOW}Creating installation directory...${NC}"
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/logs"
mkdir -p "$INSTALL_DIR/web"

# Copy files
echo -e "${YELLOW}Copying application files...${NC}"
if [ -f "bin/dashboard" ]; then
    cp bin/dashboard "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/dashboard"
else
    echo -e "${RED}Binary not found. Please run 'make build' first.${NC}"
    exit 1
fi

cp config.toml "$INSTALL_DIR/config.toml.example"
cp -r web/* "$INSTALL_DIR/web/"

# Copy configuration if it doesn't exist
if [ ! -f "$INSTALL_DIR/config.toml" ]; then
    cp config.toml "$INSTALL_DIR/config.toml"
    echo -e "${YELLOW}Created default configuration. Please edit $INSTALL_DIR/config.toml${NC}"
fi

# Set permissions
echo -e "${YELLOW}Setting permissions...${NC}"
chown -R "$USER:$GROUP" "$INSTALL_DIR"
chmod 755 "$INSTALL_DIR"
chmod 755 "$INSTALL_DIR/dashboard"
chmod 644 "$INSTALL_DIR/config.toml"
chmod 755 "$INSTALL_DIR/logs"

# Install systemd service
echo -e "${YELLOW}Installing systemd service...${NC}"
cp ecobox-server.service "$SERVICE_FILE"
systemctl daemon-reload

# Enable and start service
echo -e "${YELLOW}Enabling and starting service...${NC}"
systemctl enable ecobox-server
systemctl start ecobox-server

# Check service status
sleep 2
if systemctl is-active --quiet ecobox-server; then
    echo -e "${GREEN}✅ Network Dashboard installed and started successfully!${NC}"
    echo ""
    echo "Service status: $(systemctl is-active ecobox-server)"
    echo "Web interface: http://localhost:8080"
    echo ""
    echo "Commands:"
    echo "  sudo systemctl status ecobox-server    # Check status"
    echo "  sudo systemctl restart ecobox-server   # Restart service"
    echo "  sudo systemctl stop ecobox-server      # Stop service"
    echo "  sudo journalctl -u ecobox-server -f    # View logs"
    echo ""
    echo "Configuration: $INSTALL_DIR/config.toml"
    echo "Logs: $INSTALL_DIR/logs/"
else
    echo -e "${RED}❌ Service failed to start. Check logs with:${NC}"
    echo "sudo journalctl -u ecobox-server -f"
    exit 1
fi
