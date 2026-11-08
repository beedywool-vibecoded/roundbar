# roundbar

A round touchscreen macro controller for the desktop: a Guition JC3636W518
(ESP32-S3, 1.8" 360×360 touch) that switches its controls to match the focused app.

- **firmware/** – ESP-IDF + LVGL. Material 3 Expressive pages on the round screen
  (radial buttons, now playing, system gauges, bezel dials). Shows up as a USB
  keyboard, so shortcuts work even without the PC side, plus a CDC serial link.
- **cmd/roundbar, internal/** – one Go binary.
  - `roundbar daemon`: systemd user service bridging the device and the desktop.
    Event-driven; idles at ~0 % CPU and ~15 MB RSS.
  - `roundbar config`: serves the configurator on 127.0.0.1 while a tab is open.
- **web/** – the configurator (Vue 3, Material You from one seed color).
- **gnome-extension/** – tiny GNOME Shell extension publishing the focused app on
  D-Bus. GNOME on Wayland gives no other way to know the active window.
- **docs/spec.md** – the contract between all of the above.

## Build & install

```sh
make install          # builds web + Go, installs ~/.local/bin/roundbar, service, extension, launcher
```

Then log out and back in once so GNOME loads the extension. Open **roundbar** from
the app grid (or run `roundbar config`) to configure.

## Firmware

Needs ESP-IDF v6 (`. ~/esp/esp-idf-v6.0.3/export.sh`) and access to the serial port:

```sh
sudo cp deploy/70-roundbar.rules /etc/udev/rules.d/ && sudo udevadm control --reload && sudo udevadm trigger -s tty
```

```sh
make flash
```

Once roundbar runs on the board, `make flash` asks it to reboot into USB download
mode first. Fallback: hold BOOT while plugging it in.

Board pins: QSPI LCD SCK 9, D0–D3 11–14, CS 10, RST 47, backlight 15;
touch I²C SDA 7, SCL 8, INT 41, RST 40.
