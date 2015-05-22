package main

import (
	"fmt"
	"os"
	"time"

	"github.com/nicholaskh/golib/locking"
	"github.com/nicholaskh/golib/server"
	"github.com/nicholaskh/piped/config"
	"github.com/nicholaskh/piped/engine"
)

func init() {
	parseFlags()

	if options.showVersion {
		server.ShowVersionAndExit()
	}

	if options.kill {
		if err := server.KillProcess(options.lockFile); err != nil {
			fmt.Fprintf(os.Stderr, "stop failed: %s\n", err)
			os.Exit(1)
		}

		os.Exit(0)
	}

	server.SetupLogging(options.logFile, options.logLevel, options.crashLogFile)

	if options.lockFile != "" {
		if locking.InstanceLocked(options.lockFile) {
			fmt.Fprintf(os.Stderr, "Another piped is running, exit...\n")
			os.Exit(1)
		}

		locking.LockInstance(options.lockFile)
	}

	conf := server.LoadConfig(options.configFile)
	config.PipedConf = new(config.PipedConfig)
	config.PipedConf.LoadConfig(conf)
}

func main() {
	go server.RunSysStats(time.Now(), time.Duration(options.tick)*time.Second)

	piped := engine.NewPiped(config.PipedConf)
	piped.RunForever()
}
