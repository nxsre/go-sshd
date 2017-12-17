package server

import (
	"fmt"
	"io"
	"io/ioutil"
	// "io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	// "github.com/chzyer/readline"
	"github.com/chzyer/readline"
	"github.com/soopsio/liner"
	// "github.com/google/shlex"
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
		w_cp, // copy 到 readline 进行解析
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

	// 处理所有 shell 输出（用来审计等）
	go ProcessOutput(s, stdout_r)
	defer stdout_r.Close()

	// 使用 readline 拦截输入
	multiw := io.MultiWriter(s, stdout_w)
	go func() {
		history_fn := filepath.Join(os.TempDir(), ".liner_example_history")
		hole_file, _ := ioutil.TempFile("/tmp/", "tmp_hole_")

		line := liner.NewLiner(&liner.Config{Stdin: r_cp, Stdout: hole_file})
		defer line.Close()

		line.SetCtrlCAborts(true)

		// line.SetCompleter(func(line string) (c []string) {
		// 	for _, n := range names {
		// 		if strings.HasPrefix(n, strings.ToLower(line)) {
		// 			c = append(c, n)
		// 		}
		// 	}
		// 	return
		// })

		if f, err := os.Open(history_fn); err == nil {
			line.ReadHistory(f)
			f.Close()
		}

		// for {
		// if name, err := line.Prompt("What is your name? "); err == nil {
		// 	log.Print("Got: ", name)
		// 	line.AppendHistory(name)
		// } else if err == liner.ErrPromptAborted {
		// 	log.Print("Aborted")
		// } else {
		// 	log.Print("Error reading line: ", err)
		// 	break
		// }

		// if f, err := os.Create(history_fn); err != nil {
		// 	log.Print("Error writing history file: ", err)
		// } else {
		// 	line.WriteHistory(f)
		// 	f.Close()
		// }
		// }

		// hole_file, err := ioutil.TempFile("/tmp/", "tmp_hole_")
		// if err == nil {
		// 	defer os.Remove(hole_file.Name())
		// 	defer hole_file.Close()
		// }

		// // 过滤输入命令
		// config := readline.Config{
		// 	Prompt:              "",
		// 	HistoryFile:         "/tmp/readline.tmp",
		// 	InterruptPrompt:     "^C",
		// 	EOFPrompt:           "exit",
		// 	ForceUseInteractive: false,
		// 	HistorySearchFold:   true,
		// 	FuncFilterInputRune: filterInput,
		// 	FuncOnWidthChanged: func(f func()) {
		// 		f()
		// 	},
		// 	Stdin:  r_cp,      // 从管道读取 ssh 远程输入
		// 	Stdout: hole_file, // 丢弃标准输出和错误输出，使用 pty 中的输出
		// 	Stderr: hole_file,
		// }
		// l, err := readline.NewEx(&config)
		// config.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		// 	// fmt.Println(line, pos, key)
		// 	return
		// })
		// if err != nil {
		// 	log.Println("readline init error: ", err)
		// 	return
		// }

		// defer l.Close()

		// for {
		// 	line, err := l.Readline()
		// 	if err == io.EOF {
		// 		log.Println("stdin are closed! ")
		// 		break
		// 	} else if err != nil {
		// 		// 忽略 Ctrl+C
		// 		if err.Error() != "Interrupt" {
		// 			log.Println("Unknown exception: ", err)
		// 			break
		// 		}
		// 	} else {
		// 		line = strings.TrimRightFunc(line, func(r rune) bool {
		// 			if r == rune(' ') || r == rune('\t') || r == rune('\n') || r == rune('\r') {
		// 				return true
		// 			}
		// 			return false
		// 		})
		// 		if len(line) == 0 {
		// 			continue
		// 		}
		// 		// log.Println("实时获取输入命令:", line)
		// 		if shl, err := shlex.Split(line); err == nil {
		// 			if len(shl) >= 1 {
		// 				// 过滤非法命令
		// 				if shl[0] == "aaa" {
		// 					s.Write([]byte("\nPermission denied !!!\n"))
		// 					f.Close()
		// 					break
		// 				}
		// 			}
		// 		}
		// 	}
		// }
		// s.Exit(0)
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
