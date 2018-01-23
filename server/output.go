package server

import (
	// "bufio"

	"io"

	"go.uber.org/zap"

	// "github.com/soopsio/liner"
	"github.com/Nerdmaster/terminal"
	// "github.com/soopsio/go-utils/strutils"
	"github.com/soopsio/ssh"
)

func ProcessOutput(s ssh.Session, r io.Reader) {
	p := terminal.NewReader(r)
	for {
		var line, err = p.ReadLine()
		if err != nil {
			// fmt.Printf("%s\r\n", err)
			break
		}
		// 调用 strutils.DeSensitization 替换敏感信息
		// logger.Info(strutils.DeSensitization(line),
		// 	zap.Any("remoteaddr", s.RemoteAddr()), zap.String("username", s.User()))

		logger.Info(line,
			zap.Any("remoteaddr", s.RemoteAddr()), zap.String("username", s.User()))
	}
}
