package main

import (
	"time"

	"github.com/nicholaskh/golib/server"
	"github.com/nicholaskh/piped/config"
	"github.com/nicholaskh/piped/engine"
)

func init() {
	parseFlags()

	if options.showVersion {
		server.ShowVersionAndExit()
	}

	server.SetupLogging(options.logFile, options.logLevel, options.crashLogFile)

	conf := server.LoadConfig(options.configFile)
	config.PipedConf = new(config.PipedConfig)
	config.PipedConf.LoadConfig(conf)
}

func main() {
	go server.RunSysStats(time.Now(), time.Duration(options.tick)*time.Second)

	piped := engine.NewPiped(config.PipedConf)
	piped.RunForever()
}
