package server

import (
	"bufio"
	"io"
	"log"

	"github.com/soopsio/ssh"
)

func ProcessOutput(s ssh.Session, r io.Reader) {
	// 处理输出

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		log.Println("output:", s.RemoteAddr(), scanner.Text())
	}

	if scanner.Err() == io.ErrClosedPipe {
		return
	}

	log.Println(scanner.Err())

	// reader := bufio.NewReader(r)
	/* go func() {
		buf := make([]byte, 0, 1024)
		for {
			l, p, err := reader.ReadLine()
			if len(l) > 0 {
				fmt.Printf("#ReadData|%d|%b|%s \n", len(l), p, l)
			}
			if err != nil {
				if err != io.ErrClosedPipe {
					log.Println(err)
				}
				break
			}
		}
	}() */
}
