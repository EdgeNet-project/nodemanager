#!/bin/sh
set -e

# Reload systemd to recognize the new service file
systemctl daemon-reload

# Enable and start the service
systemctl enable nodemanager
systemctl start nodemanager
