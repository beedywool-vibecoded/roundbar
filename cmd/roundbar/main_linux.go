package main

import (
	"github.com/beedywool-vibecoded/roundbar/internal/daemon"
)

const usage = `usage: roundbar <command>

  daemon       run the bridge between the device and the desktop (systemd user service)
  config       open the configurator in the browser (--no-open to only print the URL)
  status       print daemon and device status
  firmware     check for or install a firmware release (roundbar firmware check|update)
  bootloader   reboot the device into USB download mode, for flashing
  screenshot   save what the device shows as a PNG (roundbar screenshot out.png)
  install      install the udev rule, systemd user service, app launcher (and on GNOME its extension)
               (--no-udev to skip the udev rule)
  udev         install only the udev rule that gives you access to the board (asks for your password)
  uninstall    remove what install set up and the binary in ~/.local/bin (--keep-binary);
               settings in ~/.config/roundbar stay
`

const startHint = "systemctl --user start roundbar"

var defaultCommand []string // a command is required

func setup(string) {}

func runDaemon() error { return daemon.New().Run() }

func platformCommand(string) (bool, error) { return false, nil }
