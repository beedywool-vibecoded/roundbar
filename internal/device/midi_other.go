//go:build !linux

package device

import "errors"

// FindMIDI: only Linux needs the MIDI way in (serial ports there may need a
// udev rule); elsewhere the serial port is always open to the user.
func FindMIDI() string { return "" }

func OpenMIDI(string) (*Port, error) { return nil, errors.New("MIDI transport is Linux-only") }
