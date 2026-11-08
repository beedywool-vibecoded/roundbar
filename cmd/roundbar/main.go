// roundbar: daemon, configurator and helper commands for the round macro controller.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/beedywool-vibecoded/roundbar/internal/configui"
	"github.com/beedywool-vibecoded/roundbar/internal/ipc"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		os.Args = append(os.Args, defaultCommand...)
	}
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	setup(os.Args[1])
	var err error
	switch os.Args[1] {
	case "daemon":
		err = runDaemon()
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
			err = fmt.Errorf("daemon not reachable (%v); start it with: %s", err, startHint)
		}
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
		err = install()
	default:
		var ok bool
		if ok, err = platformCommand(os.Args[1]); !ok {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}
