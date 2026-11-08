package stats

import (
	"os/exec"
	"syscall"
)

// childAttr makes a helper process die with the daemon.
func childAttr(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL} }

func childStarted(*exec.Cmd) {}
