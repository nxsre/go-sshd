package server

import (
	"bufio"
	"io"

	"github.com/soopsio/zlog/tools"
	"go.uber.org/zap"

	"github.com/soopsio/ssh"
)

func ProcessOutput(s ssh.Session, r io.Reader) {
	// 处理输出

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info(tools.StripCtlAndExtFromBytes(tools.StripAnsiColor(scanner.Text())), zap.Any("remoteaddr", s.RemoteAddr()), zap.String("username", s.User()))
	}

	if scanner.Err() == io.ErrClosedPipe {
		return
	}
}
