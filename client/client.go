package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
)

func main() {
	SSH("root", "111111", "10.10.89.127:2222")
}

func SSH(user, password, ip_port string) {
	PassWd := []ssh.AuthMethod{ssh.Password(password)}
	Conf := ssh.ClientConfig{User: user, Auth: PassWd,
		//需要验证服务端，不做验证返回nil就可以，点击HostKeyCallback看源码就知道了
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	Client, err := ssh.Dial("tcp", ip_port, &Conf)
	if err != nil {
		log.Fatalln(err)
	}
	defer Client.Close()

	if session, err := Client.NewSession(); err == nil {
		defer session.Close()
		// SendRequest 用途参考
		// https://github.com/golang/crypto/blob/master/ssh/session.go
		// session.SendRequest("aaaa", false, []byte("bbbb"))

		// scp 脚本到服务器
		file, err := os.Open("f:\\test.sh")
		if err != nil {
			log.Fatalln("打开文件失败:", err)
		}

		info, _ := file.Stat()
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 1024)
			w, err := session.StdinPipe()
			if err != nil {
				log.Println(err)
				return
			}
			defer w.Close()

			fmt.Fprintln(
				w,
				"C0644",
				info.Size(),
				info.Name(),
			)
			for {
				n, err := file.Read(buf)
				fmt.Fprint(w, string(buf[:n]))
				if err != nil {
					if err == io.EOF {
						return
					} else {
						panic(err)
					}
				}
			}

		}()

		if err := session.Run("/usr/bin/scp -q -r -t /mnt"); err != nil {
			if err.Error() != "Process exited with: 1. Reason was:  ()" {
				fmt.Println(err.Error())
			}
			session.Close()
			return
		} else {
			fmt.Printf(" %s 发送成功.\n", Client.RemoteAddr())
		}
		wg.Wait()
	} else {
		log.Fatalln("获取 session 失败", err)
	}

	if session, err := Client.NewSession(); err == nil {
		defer session.Close()
		// SendRequest 用途参考
		// https://github.com/golang/crypto/blob/master/ssh/session.go
		// session.SendRequest("aaaa", false, []byte("bbbb"))

		session.Stdout = os.Stdout
		session.Stdout = os.Stderr
		fmt.Println("开始执行")
		session.Run("bash /mnt/test.sh")
		fmt.Println("执行完毕")

	} else {
		log.Fatalln("获取 session 失败", err)
	}
}
