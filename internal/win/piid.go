// Package win holds the small Windows layer the daemon needs without cgo:
// raw COM and WinRT calls through vtables, COM objects implemented in Go,
// hidden windows with their own message loop, and process helpers.
// Everything but this file (pure logic, tested on every OS) is Windows-only.
package win

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"strings"
)

// GUIDString formats a GUID given as its 16 bytes in RFC 4122 order.
func guidString(b [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", binary.BigEndian.Uint32(b[0:4]), binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]), binary.BigEndian.Uint16(b[8:10]), b[10:16])
}

// winrtNamespace is the UUID namespace WinRT hashes generic instance signatures in.
var winrtNamespace = [16]byte{0x11, 0xf4, 0x7a, 0xd5, 0x7b, 0x73, 0x42, 0xc0, 0xab, 0xae, 0x87, 0x8b, 0x1e, 0x16, 0xad, 0xee}

// ParamIID computes the IID of an instantiated WinRT generic interface or
// delegate from its type signature, e.g.
// "pinterface({913337e9-11a1-4345-a3a2-4e7f956e222d};string)" (IVector<String>).
// It is a version 5 UUID: SHA-1 over the namespace and the UTF-8 signature.
func ParamIID(signature string) string {
	h := sha1.New()
	h.Write(winrtNamespace[:])
	h.Write([]byte(signature))
	var b [16]byte
	copy(b[:], h.Sum(nil))
	b[6] = b[6]&0x0f | 0x50
	b[8] = b[8]&0x3f | 0x80
	return guidString(b)
}

// Signature pieces for ParamIID.

// SigClass is a runtime class with its default interface.
func SigClass(name, defaultIID string) string {
	return "rc(" + name + ";{" + strings.ToLower(defaultIID) + "})"
}

// SigInterface is a non-generic interface.
func SigInterface(iid string) string { return "{" + strings.ToLower(iid) + "}" }

// SigGeneric instantiates a generic interface or delegate (its own piid) with args.
func SigGeneric(piid string, args ...string) string {
	return "pinterface({" + strings.ToLower(piid) + "};" + strings.Join(args, ";") + ")"
}
