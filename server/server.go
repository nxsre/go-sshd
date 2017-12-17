package server

import (
	"net"

	"github.com/soopsio/nkill"
	"github.com/soopsio/ssh"
)

func NewSshServer() *ssh.Server {
	sshHandler := func(s ssh.Session) {
		if s.Subsystem("sftp") {
			sftpServerStart(s)
		} else {
			ptyReq, winCh, isPty := s.Pty()
			if isPty {
				ptyStart(ptyReq, winCh, s)
			} else {
				switch s.Command()[0] {
				case "scp", "/usr/bin/scp":
					scpconfig := initScpServer("")
					scpconfig.scpServerStart(s)
				case "rsync":
					rsyncStart(s)
				default:
					cmdStart(s)
				}
			}
		}

	}

	passHandler := func(ctx ssh.Context, password string) bool {
		// fmt.Println("密码验证请求:", ctx, password)
		return true
	}

	connCallback := func(net net.Conn) net.Conn {
		return net
	}
	nkill.KillPort(2022)
	return &ssh.Server{
		Addr:            ":2022",
		Handler:         sshHandler,
		PasswordHandler: passHandler,
		ConnCallback:    connCallback,
	}
}
