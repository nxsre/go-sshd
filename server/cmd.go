package server

import (
	"context"
	"io"
	"log"
	"os/exec"
	"syscall"

	"github.com/soopsio/ssh"
)

func cmdStart(s ssh.Session) {
	log.Println("no pty:", s.Command())
	// cmd := exec.Command(s.Command()[0], s.Command()[1:]...)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, s.Command()[0], s.Command()[1:]...)
	// 端开连接时调用 cancel 退出 cmd, 然后等待 cmd 退出, 避免造成 defunct 进程
	defer cmd.Wait()
	defer cancel()

	stdout_r, stdout_w := io.Pipe()
	go ProcessOutput(s, stdout_r)
	defer stdout_r.Close()
	mwer := io.MultiWriter(stdout_w, s)
	cmd.Stdout = mwer
	cmd.Stderr = mwer
	err := cmd.Run()
	if err != nil {
		io.WriteString(s, err.Error())
	}
	var exitCode int
	if cmd != nil && cmd.ProcessState.Exited() {
		if waitStatus, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			exitCode = int(waitStatus.ExitStatus())
		}
	}
	s.Exit(exitCode)
}
