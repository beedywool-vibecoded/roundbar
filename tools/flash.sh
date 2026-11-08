#!/usr/bin/env bash
# Flash the firmware. Finds the ESP32-S3 download port whether the board runs
# roundbar (asked to reboot into the ROM USB download mode) or anything else.
set -euo pipefail
cd "$(dirname "$0")/../firmware/build"

# Any Espressif tty that isn't roundbar itself (303a:4011): USB-Serial-JTAG
# (303a:1001) or the ROM's USB-OTG download port.
dl_port() {
  for t in /sys/class/tty/ttyACM*; do
    [[ -e $t ]] || continue
    [[ $(cat "$t/device/../idVendor" 2>/dev/null) == 303a ]] || continue
    [[ $(cat "$t/device/../idProduct" 2>/dev/null) == 4011 ]] && continue
    echo "/dev/$(basename "$t")"; return
  done
}

port=$(dl_port || true)
if [[ -z $port ]]; then
  echo "asking roundbar to reboot into download mode…"
  roundbar bootloader 2>/dev/null || true
  for _ in $(seq 1 100); do port=$(dl_port || true); [[ -n $port ]] && break; sleep 0.05; done
fi
if [[ -z $port ]]; then
  echo "waiting for the board: unplug it and plug it back in (Ctrl-C to give up)…"
  until port=$(dl_port) && [[ -n $port ]]; do sleep 0.02; done
fi
echo "flashing via $port"
python -m esptool --chip esp32s3 -p "$port" --before default-reset --after hard-reset write-flash @flash_args
