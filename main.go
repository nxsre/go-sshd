package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	sshd "github.com/soopsio/go-sshd/server"

	"context"

	"github.com/soopsio/zlog"
	"github.com/soopsio/zlog/zlogbeat/cmd"
	"go.uber.org/config"
	"go.uber.org/zap"
)

var (
	cfgfile = flag.String("logconf", "conf/log.yml", "main log config file.")
)

func initLogger() {
	cmd.RootCmd.Flags().AddGoFlag(flag.CommandLine.Lookup("logconf"))
	flag.Parse()
	p, err := config.NewYAMLProviderFromFiles(*cfgfile)
	if err != nil {
		log.Fatalln(err)
	}

	sw := zlog.NewWriteSyncer(p)
	conf := zap.NewProductionConfig()
	conf.DisableCaller = true
	conf.Encoding = "json"

	logger, _ := conf.Build(zlog.SetOutput(sw, conf))

	sshd.SetLogger(logger)
}

var c chan os.Signal

func main() {
	initLogger()
	c = make(chan os.Signal, 1)

	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	server := sshd.NewSshServer()
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
	LOOP:
		for {
			select {
			case s := <-c:
				fmt.Println("SSHD | get", s)
				server.Shutdown(ctx)
				break LOOP
			default:
				time.Sleep(500 * time.Millisecond)
			}
		}
		cancel()
		// err := server.Close()
		fmt.Println("SSHD | close ctx, exit")
	}()
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalln(err)
	}

}
