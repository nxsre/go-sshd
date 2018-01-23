package server

import (
	"fmt"
	"io"
	"io/ioutil"
	// "io/ioutil"
	"log"
	"time"
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
	"github.com/soopsio/gors/output"
	"github.com/soopsio/ssh"
	"go.uber.org/zap"
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
	defer func() {
		var exitCode int
		processState, err := cmd.Process.Wait()
		if err != nil {
			io.WriteString(s, err.Error())
		}

		if processState != nil && processState.Exited() {
			// 获取退出状态
			if waitStatus, ok := processState.Sys().(syscall.WaitStatus); ok {
				exitCode = int(waitStatus.ExitStatus())
			}
		}
		s.Exit(exitCode)
		s.Close()
	}()
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

	var windowSize = &struct {
		width  int64
		height int64
	}{}
	go func() {
		for win := range winCh {
			setWinsize(f, win.Width, win.Height)
			windowSize.height = int64(win.Height)
			windowSize.width = int64(win.Width)
		}
	}()

	// 直接 copy 流
	// 录屏
	// 存放终端录屏的输出
	bufferOutput := output.NewOutput(1000)
	// 录屏
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
	multiw := io.MultiWriter(s, stdout_w, bufferOutput)
	go func() {
		history_fn := filepath.Join(os.TempDir(), ".liner_example_history")
		hole_file, _ := ioutil.TempFile("/tmp/", "tmp_hole_")

		line := liner.NewLiner(&liner.Config{Stdin: r_cp, Stdout: hole_file})
		defer line.Close()

		line.SetCtrlCAborts(true)

		if f, err := os.Open(history_fn); err == nil {
			line.ReadHistory(f)
			f.Close()
		}
	}()

	io.Copy(multiw, f) // stdout

	// 录屏
	recordFilename := filepath.Join("/usr/share/",time.Now().Format("2006-01-02"),s.User()+"_"+time.Now().Format("20060102_150405"))
	if err:=os.MkdirAll(filepath.Dir(recordFilename),0755);err!=nil{
		logger.Error("can not create logdir "+filepath.Dir(recordFilename), zap.Error(err))
		return
	}
	recordFile, err := os.Create(recordFilename)
	if err != nil {
		logger.Error("can not create filename "+recordFilename, zap.Error(err))
		return
	}
	defer recordFile.Close()
	dest := output.NewDestination(bufferOutput, "0.0.1", windowSize.width, windowSize.height, "/bin/bash", "Gors Record", os.Getenv("TERM"), os.Getenv("SHELL"))
	if err := dest.Save(recordFile); err != nil {
		logger.Error("save to file has a error", zap.Error(err))
	}
	// 录屏
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
