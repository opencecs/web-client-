#!/bin/bash
# Debian (systemd) install script
set -e

echo "=== Installing myt-panel service (Debian/systemd) ==="

# copy service file
cp debian-systemd.service /etc/systemd/system/myt-panel.service

# reload and enable
systemctl daemon-reload
systemctl enable myt-panel
systemctl start myt-panel

echo "Done! Check status: systemctl status myt-panel"
