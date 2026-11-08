#!/bin/sh
# .deb/.rpm post-install: apply the udev rule to a board that is already plugged in.
if command -v udevadm >/dev/null 2>&1; then
	udevadm control --reload || true
	udevadm trigger --subsystem-match=tty --action=change || true
fi
cat <<'EOF'
roundbar: for each desktop user, run `roundbar install` once (enables the
user service and the GNOME extension), then open "roundbar" from the app launcher.
EOF
