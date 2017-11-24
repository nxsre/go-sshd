package main

import (
	"log"
	"net/http"

	sshd "github.com/soopsio/go-sshd/server"

	_ "net/http/pprof"
)

func main() {
	//远程获取pprof数据
	go func() {
		log.Println(http.ListenAndServe(":8080", nil))
	}()

	s := sshd.NewSshServer()
	err := s.ListenAndServe()
	if err != nil {
		log.Fatalln(err)
	}
}
