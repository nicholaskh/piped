package engine

import (
	"time"

	"github.com/nicholaskh/golib/server"
	"github.com/nicholaskh/piped/config"
	"github.com/nicholaskh/piped/engine/alarmer"
	"github.com/nicholaskh/piped/engine/analyser"
	"github.com/nicholaskh/piped/engine/flusher"
)

type Piped struct {
	config          *config.PipedConfig
	server          *server.TcpServer
	serverStats     *ServerStats
	clientProcessor *PipedClientProcessor
	flusher         *flusher.Flusher
	alarmer         *alarmer.Alarmer
	analyser        *analyser.Analyser
}

func NewPiped(config *config.PipedConfig) *Piped {
	this := new(Piped)
	this.config = config
	this.server = server.NewTcpServer("piped")
	this.serverStats = NewServerStats()

	var flushInterval time.Duration
	if this.config.Analyser.StatsCountInterval > this.config.Analyser.ElapsedCountInterval {
		flushInterval = this.config.Analyser.StatsCountInterval
	} else {
		flushInterval = this.config.Analyser.ElapsedCountInterval
	}
	if flushInterval < this.config.Analyser.AlarmCountInterval {
		flushInterval = this.config.Analyser.AlarmCountInterval
	}

	this.flusher = flusher.NewFlusher(config.Mongo, config.Flusher, flushInterval)
	this.alarmer = alarmer.NewAlarmer(config.Alarm)
	this.analyser = analyser.NewAnalyser(config.Analyser, config.Mongo, this.flusher, this.alarmer)
	this.clientProcessor = NewPipedClientProcessor(this.server, this.serverStats, this.flusher, this.alarmer, this.analyser)

	this.flusher.RegisterStats(this.analyser.ElapsedStats)
	this.flusher.RegisterAlarmStats(this.analyser.AlarmStats)
	this.flusher.RegisterXapiStats(this.analyser.XapiStats)

	return this
}

func (this *Piped) RunForever() {
	go server.StartPingServer(this.config.UdpPort)

	go this.server.LaunchTcpServer(this.config.ListenAddr, this.clientProcessor, this.config.SessionTimeout, 5)
	go this.serverStats.Start(this.config.StatsOutputInterval, this.config.MetricsLogfile)

	this.flusher.Serv()
	this.alarmer.Serv()
	this.analyser.Serv()

	done := make(chan int)
	<-done
}
