# roundbar: `make` builds the configurator UI and the Go binary,
# `make install` puts the binary in ~/.local/bin and sets up the service,
# `make firmware flash` needs ESP-IDF (`. ~/esp/esp-idf-v6.0.3/export.sh`).
BIN := bin/roundbar
PREFIX ?= $(HOME)/.local

.PHONY: all web go windows install test firmware flash monitor

all: web go

web:
	cd web && npm install --no-audit --no-fund && npm run build

go:
	go build -trimpath -ldflags="-s -w -X github.com/beedywool-vibecoded/roundbar/internal/configui.RepoDir=$(CURDIR)" -o $(BIN) ./cmd/roundbar

# The Windows tray app (cross-compiled, no cgo) and, if makensis is installed, its installer.
windows:
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w -H=windowsgui" -o bin/roundbar.exe ./cmd/roundbar
	go run ./tools/winicon -o deploy/windows/roundbar.ico
	@if command -v makensis >/dev/null; then \
		makensis -V2 -DEXE=$(CURDIR)/bin/roundbar.exe -DOUTFILE=$(CURDIR)/bin/roundbar-setup-windows-amd64.exe deploy/windows/roundbar.nsi; \
	else echo "makensis not found: built bin/roundbar.exe only"; fi

test:
	go test ./...

install: all
	install -Dm755 $(BIN) $(PREFIX)/bin/roundbar
	$(PREFIX)/bin/roundbar install

firmware:
	cd firmware && idf.py set-target esp32s3 >/dev/null 2>&1 || true; cd firmware && idf.py build

# Flashes the board. If our firmware is running, it is first asked to reboot into
# the USB download mode; the Espressif USB-JTAG port then shows up.
flash: firmware
	./tools/flash.sh
