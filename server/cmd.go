package server

import (
	"context"
	"io"
	"os/exec"
	"syscall"

	"go.uber.org/zap"

	"github.com/soopsio/ssh"
)

func cmdStart(s ssh.Session) {
	logger.Info("no pty:", zap.Strings("commands", s.CommandRaw()))
	// cmd := exec.Command(s.Command()[0], s.Command()[1:]...)
	ctx, cancel := context.WithCancel(context.Background())
	// cmd := exec.CommandContext(ctx, s.Command()[0], s.Command()[1:]...)
	cmd := exec.CommandContext(ctx, "bash", "-c", s.CommandRaw())
	// 端开连接时调用 cancel 退出 cmd, 然后等待 cmd 退出, 避免造成 defunct 进程
	// defer cmd.Wait()
	defer cancel()

	stdout_r, stdout_w := io.Pipe()
	go ProcessOutput(s, stdout_r)
	defer stdout_r.Close()
	mwer := io.MultiWriter(stdout_w, s)
	cmd.Stdout = mwer
	cmd.Stderr = mwer
	err := cmd.Start()
	var exitCode int
	if err != nil {
		io.WriteString(s, err.Error())
		if e2, ok := err.(*exec.Error); ok {
			if e2.Err == exec.ErrNotFound {
				exitCode = 127
			}
		}
		s.Exit(exitCode)
		s.Close()
		return
	}

	processState, err := cmd.Process.Wait()
	if err != nil {
		io.WriteString(s, err.Error())
	}

	if processState != nil && processState.Exited() {
		// 获取退出状态
		if waitStatus, ok := processState.Sys().(syscall.WaitStatus); ok {
			exitCode = int(waitStatus.ExitStatus())
		}
	}
	s.Exit(exitCode)
	s.Close()
	logger.Info("no pty cmd over:", zap.Strings("commands", s.CommandRaw()))
}
