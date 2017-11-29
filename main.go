package main

import (
	"flag"
	"log"

	sshd "github.com/soopsio/go-sshd/server"

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

func main() {
	// initLogger()
	s := sshd.NewSshServer()
	err := s.ListenAndServe()
	if err != nil {
		log.Fatalln(err)
	}
}
