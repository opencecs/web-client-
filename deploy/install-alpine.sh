#!/bin/bash
# Alpine (OpenRC) install script
set -e

echo "=== Installing myt-panel service (Alpine/OpenRC) ==="

# copy service file
cp alpine-openrc /etc/init.d/myt-panel
chmod +x /etc/init.d/myt-panel

# enable on boot
rc-update add myt-panel default

# start service
rc-service myt-panel start

echo "Done! Check status: rc-service myt-panel status"
