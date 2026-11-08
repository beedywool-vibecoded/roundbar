package discord

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/windows"
)

// pipeConn is a named pipe client as a net.Conn. The pipe is opened for
// overlapped I/O, so the Go runtime reads and writes it concurrently and
// deadlines work.
type pipeConn struct{ *os.File }

type pipeAddr string

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return string(a) }

func (c pipeConn) LocalAddr() net.Addr  { return pipeAddr(c.Name()) }
func (c pipeConn) RemoteAddr() net.Addr { return pipeAddr(c.Name()) }

// dial connects to Discord's RPC pipe, \\.\pipe\discord-ipc-<0..9>.
func dial() (net.Conn, error) {
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
		if err == nil {
			return pipeConn{os.NewFile(uintptr(h), name)}, nil
		}
	}
	return nil, errors.New("Discord isn't running")
}
