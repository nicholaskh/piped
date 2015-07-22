package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/nicholaskh/etclib"
	"github.com/nicholaskh/golib/locking"
	"github.com/nicholaskh/golib/server"
	"github.com/nicholaskh/golib/signal"
	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
	"github.com/nicholaskh/piped/engine"
)

func init() {
	parseFlags()

	if options.showVersion {
		server.ShowVersionAndExit()
	}

	conf := server.LoadConfig(options.configFile)
	config.PipedConf = new(config.PipedConfig)
	config.PipedConf.LoadConfig(conf)

	if options.kill {
		if err := server.KillProcess(options.lockFile); err != nil {
			fmt.Fprintf(os.Stderr, "stop failed: %s\n", err)
			os.Exit(1)
		}
		etclib.Dial(config.PipedConf.EtcServers)
		engine.LoadLocalAddr(config.PipedConf.ListenAddr)
		engine.UnregisterEtc()
		cs, _ := etclib.Children("/piped")
		fmt.Println(cs)

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

	signal.RegisterSignalHandler(syscall.SIGINT, func(sig os.Signal) {
		shutdown()
	})

	engine.LoadLocalAddr(config.PipedConf.ListenAddr)
}

func main() {
	if options.cpuprofile != "" {
		cpuprofile := "piped.prof"
		f, err := os.Create(cpuprofile)
		if err != nil {
			println(err)
		}
		pprof.StartCPUProfile(f)
	}

	defer func() {
		cleanup()

		if err := recover(); err != nil {
			fmt.Println(err)
			debug.PrintStack()
		}
	}()

	err := engine.RegisterEtc(config.PipedConf.EtcServers)
	if err != nil {
		panic(err)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	go server.RunSysStats(time.Now(), time.Duration(options.tick)*time.Second)

	piped := engine.NewPiped(config.PipedConf)
	piped.RunForever()
}

func shutdown() {
	cleanup()
	log.Info("Terminated")
	os.Exit(0)
}

func cleanup() {
	if options.lockFile != "" {
		locking.UnlockInstance(options.lockFile)
		log.Debug("Cleanup lock %s", options.lockFile)
	}
	if options.cpuprofile != "" {
		pprof.StopCPUProfile()
	}
	engine.UnregisterEtc()
}
