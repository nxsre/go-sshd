package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	"github.com/chzyer/readline"
)

func main() {
	// var out io.Writer
	var in io.Reader

	in = os.NewFile(uintptr(syscall.Stdin), "/dev/stdin")
	r, w, err := os.Pipe()
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		// scanner 扫描行，缺点： 不支持 shell 完整的操作，如 Backspace
		// scanner := bufio.NewScanner(r)
		// scanner.Split(bufio.ScanLines)
		// for scanner.Scan() {
		// 	fmt.Println(scanner.Text())
		// }

		// 使用 readline 包，github.com/chzyer/readline
		readline.Stdin = r
		l, err := readline.NewEx(&readline.Config{
			Prompt:          "\033[31m»\033[0m ",
			HistoryFile:     "/tmp/readline.tmp",
			InterruptPrompt: "^C",
			EOFPrompt:       "exit",

			HistorySearchFold:   true,
			FuncFilterInputRune: filterInput,
		})
		fmt.Println("333333", err)
		for {
			line, err := l.Readline()
			if err == io.EOF {
				fmt.Println("shell 关闭")
				break
			} else if err != nil {
				log.Println("未知异常:", err)
				break
			} else {
				log.Println(line)
			}
		}
		w.Close()
		r.Close()
		l.Clean()
		l.Close()
	}()
	n, err := io.Copy(w, in)

	fmt.Printf("\n write %d err %v \n", n, err)

}

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}
