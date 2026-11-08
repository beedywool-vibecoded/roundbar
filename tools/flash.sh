#!/usr/bin/env bash
# Flash the firmware. Works whether the board runs roundbar (TinyUSB, no auto-reset)
# or anything else exposing the ESP32-S3 USB-Serial-JTAG port.
set -euo pipefail
cd "$(dirname "$0")/../firmware"

jtag() { ls /dev/serial/by-id/usb-Espressif_USB_JTAG_serial_debug_unit_* 2>/dev/null | head -n1; }
ours() { ls /dev/serial/by-id/usb-roundbar_roundbar_* 2>/dev/null | head -n1; }

port=$(jtag || true)
if [[ -z "$port" ]]; then
  if [[ -n "$(ours || true)" ]]; then
    echo "asking roundbar to reboot into download mode…"
    roundbar bootloader 2>/dev/null || stty -F "$(ours)" 1200 || true
  fi
  for _ in $(seq 1 50); do port=$(jtag || true); [[ -n "$port" ]] && break; sleep 0.2; done
fi
if [[ -z "$port" ]]; then
  echo "no ESP32-S3 download port found. Hold BOOT, tap RESET (or replug), release BOOT, retry." >&2
  exit 1
fi
echo "flashing via $port"
idf.py -p "$port" flash
