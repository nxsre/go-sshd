package server

import (
	"fmt"
	"net"

	"github.com/soopsio/ssh"
)

var (
	sshHandler = func(s ssh.Session) {
		if s.Subsystem("sftp") {
			sftpServerStart(s)
		} else {
			ptyReq, winCh, isPty := s.Pty()
			if isPty {
				ptyStart(ptyReq, winCh, s)
			} else {
				switch s.Command()[0] {
				case "scp", "/usr/bin/scp":
					scpconfig := initScpServer("/")
					scpconfig.scpServerStart(s)
				case "rsync":
					rsyncStart(s)
				default:
					cmdStart(s)
				}
			}
		}

	}

	passHandler = func(ctx ssh.Context, password string) bool {
		fmt.Println("密码验证请求:", ctx, password)
		return true
	}

	connCallback = func(net net.Conn) net.Conn {

		return net
	}

	s = &ssh.Server{
		Addr:            ":2222",
		Handler:         sshHandler,
		PasswordHandler: passHandler,
		ConnCallback:    connCallback,
	}
)

func Start() error {
	return s.ListenAndServe()
}
