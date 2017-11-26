package main

import (
	"log"
	"net"

	"github.com/soopsio/go-sshd/scp"
	// "github.com/hnakamur/go-scp"
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
	log.Println("登录服务器")
	client, err := ssh.Dial("tcp", ip_port, &Conf)
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	/* log.Println("发送文件")
	if err := scp.NewSCP(client).SendFile("./test.sh", "/mnt"); err != nil {
		log.Fatalln(err)
	}

	log.Println("执行脚本")
	func() {
		if session, err := client.NewSession(); err == nil {
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
	}()
	err = scp.NewSCP(client).ReceiveFile("/mnt/test.sh", "./test_1111.sh")
	log.Println(err) */

	log.Println("传输目录")
	err = scp.NewSCP(client).SendDir("/tmp/a", "/tmp/b", nil)
	log.Println(err)

}
