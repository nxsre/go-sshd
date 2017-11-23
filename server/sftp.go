package server

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pkg/sftp"
	"github.com/soopsio/ssh"
)

func sftpServerStart(s ssh.Session) {
	// 启动 sftp Server
	debugStream := ioutil.Discard
	debugStream = os.Stdout
	server, err := sftp.NewServer(
		s,
		sftp.WithDebug(debugStream),
	)
	if err != nil {
		fmt.Println("new sftp server error: ", err)
	}
	if err := server.Serve(); err != nil {
		fmt.Println("sftp serve error: ", err)
	}
}
