package server

import (
	"context"
	"io"
	"log"
	"os/exec"
	"syscall"

	"github.com/soopsio/ssh"
)

func rsyncStart(s ssh.Session) {
	var exitCode int
	defer s.Exit(exitCode)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, s.Command()[0], s.Command()[1:]...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	input, _ := cmd.StdinPipe()
	err := cmd.Start()
	if err != nil {
		s.Stderr().Write([]byte(err.Error() + "\n"))
		log.Println("rsync start failed:", err)
		return
	}
	defer cancel()

	go io.Copy(s, stdout)
	go io.Copy(s.Stderr(), stderr)

	writer := io.MultiWriter(input)
	go io.Copy(writer, s)

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				log.Printf("Exit Status: %d", status.ExitStatus())
				exitCode = int(status.ExitStatus())
			}
		} else {
			// log.Fatalf("rsynccmd.Wait: %v", err)
			log.Printf("rsynccmd.Wait: %v", err)
		}
	}
}
