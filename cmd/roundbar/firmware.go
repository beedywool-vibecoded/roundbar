package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/beedywool-vibecoded/roundbar/internal/fwupdate"
	"github.com/beedywool-vibecoded/roundbar/internal/ipc"
)

const firmwareUsage = `usage: roundbar firmware <check|update> [options]

  check               compare the device's firmware with the latest release
  update [--yes]      install the latest release on the device
  update --from DIR   install a local release directory (tools/fwrelease output)
`

func deviceFirmware() (string, bool) {
	var st struct {
		Device struct {
			Connected bool   `json:"connected"`
			FW        string `json:"fw"`
		} `json:"device"`
	}
	if ipc.Call(ipc.Request{Op: "status"}, &st) != nil {
		return "", false
	}
	return st.Device.FW, st.Device.Connected
}

func firmwareCmd(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, firmwareUsage)
		os.Exit(2)
	}
	yes, from := false, ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--yes", "-y":
			yes = true
		case "--from":
			if i+1 >= len(args) {
				return errors.New("--from needs a directory")
			}
			i++
			from = args[i]
		default:
			return fmt.Errorf("unknown option %s", args[i])
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	f := fwupdate.NewFetcher()
	var m *fwupdate.Manifest
	var err error
	if from != "" {
		raw, err := os.ReadFile(filepath.Join(from, fwupdate.ManifestFile))
		if err != nil {
			return err
		}
		if m, err = fwupdate.ParseManifest(raw); err != nil {
			return err
		}
	} else if m, err = f.Latest(ctx); err != nil {
		return fmt.Errorf("couldn't get the latest release: %w", err)
	}
	fw, connected := deviceFirmware()
	status := fwupdate.Unknown
	if connected {
		status = fwupdate.Compare(fw, m.Version)
	} else {
		fw = "not connected"
	}
	fmt.Printf("device:  %s\nrelease: %s\n", fw, m.Version)

	switch args[0] {
	case "check":
		switch status {
		case fwupdate.UpToDate:
			fmt.Println("up to date")
		case fwupdate.UpdateAvailable:
			fmt.Println("update available: roundbar firmware update")
		case fwupdate.DeviceNewer:
			fmt.Println("the device runs a newer build than the release")
		}
		return nil
	case "update":
	default:
		fmt.Fprint(os.Stderr, firmwareUsage)
		os.Exit(2)
	}

	if !yes {
		q := "Install " + m.Version + " on the device? Don't unplug it during the update."
		if ok, asked := ask(q, true); asked && !ok {
			return nil
		}
	}
	u := fwupdate.New(m, f, func() error { return ipc.Call(ipc.Request{Op: "bootloader"}, nil) })
	if from != "" {
		u.Part = func(_ context.Context, p fwupdate.Part) ([]byte, error) {
			b, err := os.ReadFile(filepath.Join(from, p.File))
			if err != nil {
				return nil, err
			}
			return b, p.Verify(b)
		}
	}
	last := -1
	u.Log = func(s string) {
		if last >= 0 {
			fmt.Println()
			last = -1
		}
		fmt.Println(s)
	}
	u.Progress = func(done, total int) {
		if pct := done * 100 / max(total, 1); pct != last {
			last = pct
			fmt.Printf("\rwriting… %3d %%", pct)
		}
	}
	if err := u.Run(ctx); err != nil {
		if last >= 0 {
			fmt.Println()
		}
		return err
	}
	if last >= 0 {
		fmt.Println()
	}
	if from == "" {
		f.Prune(m)
	}
	fmt.Println("done: the device restarts with", m.Version)
	return nil
}
