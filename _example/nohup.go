package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "/root/test.sh")
	// 端开连接时调用 cancel 退出 cmd, 然后等待 cmd 退出, 避免造成 defunct 进程

	defer cancel()

	stdout_r, stdout_w := io.Pipe()
	go ProcessOutput(stdout_r)
	defer stdout_r.Close()
	mwer := io.MultiWriter(stdout_w)
	cmd.Stdout = mwer
	cmd.Stderr = mwer
	fmt.Println("going running")
	err := cmd.Start()
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println("start running")

	// cmd.Wait()
	p, err := cmd.Process.Wait()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("finish", p)

	var exitCode int
	if cmd != nil && p.Exited() {
		if waitStatus, ok := p.Sys().(syscall.WaitStatus); ok {
			exitCode = int(waitStatus.ExitStatus())
		}
	}
	fmt.Println("test", exitCode)
}

func ProcessOutput(r io.Reader) {
	bufr := bufio.NewReader(r)
	for {
		l, _, err := bufr.ReadLine()
		if err != nil {
			return
		}
		fmt.Println(string(l))
	}
}
