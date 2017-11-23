package server

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"github.com/chzyer/readline"
	"github.com/google/shlex"
	"github.com/kr/pty"
	"github.com/shirou/gopsutil/process"
	"github.com/soopsio/ssh"
)

func ptyStart(ptyReq ssh.Pty, winCh <-chan ssh.Window, s ssh.Session) {
	cmd := exec.Command("bash")
	cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
	cmd.Env = append(cmd.Env, os.Environ()...)
	f, err := pty.Start(cmd)
	if err != nil {
		log.Println(err)
		return
	}
	defer cmd.Wait()
	proc, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		log.Println(err)
		return
	}

	defer func() {
		childs, _ := proc.Children()
		if len(childs) > 0 {
			killAll(childs)
		}
		log.Println(proc.Kill())
	}()

	go func() {
		for win := range winCh {
			setWinsize(f, win.Width, win.Height)
		}
	}()

	// 直接 copy 流
	r, w, _ := os.Pipe()
	r_cp, w_cp, _ := os.Pipe()
	mw := []io.Writer{
		w,
		w_cp,
	}
	mwer := io.MultiWriter(mw...)
	defer r.Close()
	defer w.Close()
	defer r_cp.Close()
	defer w_cp.Close()
	go func() {
		io.Copy(mwer, s) // stdin
	}()
	go func() {
		io.Copy(f, r) // shell
	}()

	stdout_r, stdout_w := io.Pipe()
	go ProcessOutput(s, stdout_r)
	defer stdout_r.Close()

	// 使用 readline 拦截输入
	multiw := io.MultiWriter(s, stdout_w)
	go func() {
		hole_file, err := ioutil.TempFile("/tmp/", "tmp_hole_")
		if err == nil {
			defer os.Remove(hole_file.Name())
			defer hole_file.Close()
		}

		// 过滤输入命令
		config := readline.Config{
			// Prompt:              "",
			HistoryFile:         "/tmp/readline.tmp",
			InterruptPrompt:     "^C",
			EOFPrompt:           "exit",
			ForceUseInteractive: false,
			HistorySearchFold:   true,
			FuncFilterInputRune: filterInput,
			FuncOnWidthChanged: func(f func()) {
				f()
			},
			Stdin:  r_cp,      // 从管道读取 ssh 远程输入
			Stdout: hole_file, // 丢弃标准输出和错误输出，使用 pty 中的输出
			Stderr: hole_file,
		}
		l, err := readline.NewEx(&config)
		config.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
			// fmt.Println(line, pos, key)
			return
		})
		if err != nil {
			log.Println("readline init error: ", err)
			return
		}

		defer l.Close()

		for {
			line, err := l.Readline()
			if err == io.EOF {
				log.Println("stdin are closed! ")
				break
			} else if err != nil {
				// 忽略 Ctrl+C
				if err.Error() != "Interrupt" {
					log.Println("Unknown exception: ", err)
					break
				}
			} else {
				line = strings.TrimRightFunc(line, func(r rune) bool {
					if r == rune(' ') || r == rune('\t') || r == rune('\n') || r == rune('\r') {
						return true
					}
					return false
				})
				if len(line) == 0 {
					continue
				}
				log.Println("实时获取输入命令:", line)
				if shl, err := shlex.Split(line); err == nil {
					if len(shl) >= 1 {
						// 过滤非法命令
						if shl[0] == "aaa" {
							s.Write([]byte("\nPermission denied !!!\n"))
							f.Close()
							break
						}
					}
				}
			}
		}
		s.Exit(0)
	}()

	io.Copy(multiw, f) // stdout
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),

		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

func killAll(procs []*process.Process) {
	for _, p := range procs {
		cs, _ := p.Children()
		if len(cs) > 0 {
			killAll(cs)
		} else {
			pid, _ := p.Ppid()
			proc, _ := process.NewProcess(pid)
			proc.Kill()
		}
	}
}
