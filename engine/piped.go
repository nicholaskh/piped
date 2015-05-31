package engine

import (
	"time"

	"github.com/nicholaskh/golib/server"
	"github.com/nicholaskh/piped/config"
)

type Piped struct {
	config          *config.PipedConfig
	server          *server.TcpServer
	serverStats     *ServerStats
	clientProcessor *PipedClientProcessor
	flusher         *Flusher
}

func NewPiped(config *config.PipedConfig) *Piped {
	this := new(Piped)
	this.config = config
	this.server = server.NewTcpServer("piped")
	this.serverStats = NewServerStats()

	var maxInterval time.Duration
	if this.config.Stats.StatsCountInterval > this.config.Stats.ElapsedCountInterval {
		maxInterval = this.config.Stats.StatsCountInterval
	} else {
		maxInterval = this.config.Stats.ElapsedCountInterval
	}
	this.flusher = NewFlusher(this.config.Mongo, this.config.Flusher, maxInterval)
	this.clientProcessor = NewPipedClientProcessor(this.server, this.serverStats, this.flusher)

	this.flusher.RegisterStats(this.clientProcessor.logProc.Stats)

	return this
}

func (this *Piped) RunForever() {
	go server.StartPingServer(this.config.UdpPort)

	go this.server.LaunchTcpServer(this.config.ListenAddr, this.clientProcessor, this.config.SessionTimeout, 5)
	go this.serverStats.Start(this.config.StatsOutputInterval, this.config.MetricsLogfile)

	this.flusher.Serv()

	done := make(chan int)
	<-done
}
