MYT Panel Installation Guide
========================================

1. Upload files to device
----------------------------------------
scp myt-panel myt-panel.sha256 user@DEVICE_IP:/home/user/
scp -r deploy user@DEVICE_IP:/home/user/

Then SSH into device:
ssh user@DEVICE_IP


2. First time setup
----------------------------------------

  chmod +x /home/user/myt-panel

  [Alpine Linux]
    cd /home/user/deploy
    chmod +x install-alpine.sh
    sudo ./install-alpine.sh

  [Debian]
    cd /home/user/deploy
    chmod +x install-debian.sh
    sudo ./install-debian.sh


3. Service commands
----------------------------------------

  [Alpine Linux (OpenRC)]
    rc-service myt-panel start
    rc-service myt-panel stop
    rc-service myt-panel restart
    rc-service myt-panel status

  [Debian (systemd)]
    systemctl start myt-panel
    systemctl stop myt-panel
    systemctl restart myt-panel
    systemctl status myt-panel


4. Default access
----------------------------------------
  URL  : http://DEVICE_IP:8081
  User : myt
  Pass : myt


5. Custom port
----------------------------------------
Edit service file to change -port:

  [Alpine]  /etc/init.d/myt-panel
  [Debian]  /etc/systemd/system/myt-panel.service
            (then: systemctl daemon-reload && systemctl restart myt-panel)


6. Online update
----------------------------------------
Panel supports in-app update (admin only).
Go to Device Management page > Panel Version card > Check Update.

Update flow: download -> SHA256 verify -> replace binary -> auto restart.
Service manager keeps it running, no manual action needed.


7. Manual update
----------------------------------------
  rc-service myt-panel stop          # Alpine
  systemctl stop myt-panel           # Debian

  cp myt-panel /home/user/myt-panel
  cp myt-panel.sha256 /home/user/myt-panel.sha256
  chmod +x /home/user/myt-panel

  rc-service myt-panel start         # Alpine
  systemctl start myt-panel          # Debian


8. Logs
----------------------------------------
  /home/user/logs/myt-panel.log
  Auto-rotated: 50MB per file, 5 backups, 30 days retention.


9. Uninstall
----------------------------------------
  [Alpine]
    rc-service myt-panel stop
    rc-update del myt-panel
    rm /etc/init.d/myt-panel

  [Debian]
    systemctl stop myt-panel
    systemctl disable myt-panel
    rm /etc/systemd/system/myt-panel.service
    systemctl daemon-reload

  rm /home/user/myt-panel
  rm /home/user/myt-panel.sha256
  rm -rf /home/user/logs
