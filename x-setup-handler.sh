#!/bin/bash

# Setup iron url handler (Linux)

set -e

cat > ~/.local/share/applications/iron-handler.desktop <<EOF
[Desktop Entry]
Type=Application
Name=Iron Scheme Handler
Exec=iron x-open %u
Terminal=true
MimeType=x-scheme-handler/iron;
EOF

xdg-mime default iron-handler.desktop x-scheme-handler/iron
