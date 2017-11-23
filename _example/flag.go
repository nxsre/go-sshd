package main

import (
	"fmt"

	"github.com/jessevdk/go-flags"
)

type Opts struct {
	To bool `short:"t"`
	P  bool `short:"p"`
}

func main() {
	opts := Opts{}
	args := []string{
		"-pt",
	}
	fmt.Println(flags.ParseArgs(&opts, args))
	fmt.Println(opts)
}
