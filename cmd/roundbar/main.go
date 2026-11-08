// roundbar: daemon, configurator and helper commands for the round macro controller.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/beedywool-vibecoded/roundbar/internal/configui"
	"github.com/beedywool-vibecoded/roundbar/internal/daemon"
	"github.com/beedywool-vibecoded/roundbar/internal/ipc"
)

const usage = `usage: roundbar <command>

  daemon       run the bridge between the device and the desktop (systemd user service)
  config       open the configurator in the browser (--no-open to only print the URL)
  status       print daemon and device status
  firmware     check for or install a firmware release (roundbar firmware check|update)
  bootloader   reboot the device into USB download mode, for flashing
  screenshot   save what the device shows as a PNG (roundbar screenshot out.png)
  install      install the udev rule, systemd user service, GNOME extension and app launcher
               (--no-udev to skip the udev rule)
  udev         install only the udev rule that gives you access to the board (asks for your password)
  uninstall    remove what install set up and the binary in ~/.local/bin (--keep-binary);
               settings in ~/.config/roundbar stay
`

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "daemon":
		err = daemon.New().Run()
	case "config":
		open := !(len(os.Args) > 2 && os.Args[2] == "--no-open")
		err = configui.Run(open)
	case "status":
		var st json.RawMessage
		if err = ipc.Call(ipc.Request{Op: "status"}, &st); err == nil {
			var pretty map[string]any
			_ = json.Unmarshal(st, &pretty)
			b, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Println(string(b))
		} else {
			err = fmt.Errorf("daemon not reachable (%v); start it with: systemctl --user start roundbar", err)
		}
	case "firmware":
		err = firmwareCmd(os.Args[2:])
	case "bootloader":
		err = ipc.Call(ipc.Request{Op: "bootloader"}, nil)
	case "screenshot":
		out := "roundbar.png"
		if len(os.Args) > 2 {
			out = os.Args[2]
		}
		var r struct {
			PNG []byte `json:"png"`
		}
		if err = ipc.Call(ipc.Request{Op: "screenshot"}, &r); err == nil {
			err = os.WriteFile(out, r.PNG, 0o644)
		}
	case "install":
		err = install(os.Args[2:])
	case "udev":
		yes := len(os.Args) > 2 && (os.Args[2] == "--yes" || os.Args[2] == "-y")
		err = installUdev(!yes)
	case "uninstall":
		err = uninstall(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}
