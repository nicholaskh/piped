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
	alarmer         *Alarmer
}

func NewPiped(config *config.PipedConfig) *Piped {
	this := new(Piped)
	this.config = config
	this.server = server.NewTcpServer("piped")
	this.serverStats = NewServerStats()

	var flushInterval time.Duration
	if this.config.Stats.StatsCountInterval > this.config.Stats.ElapsedCountInterval {
		flushInterval = this.config.Stats.StatsCountInterval
	} else {
		flushInterval = this.config.Stats.ElapsedCountInterval
	}
	if flushInterval < this.config.Stats.AlarmCountInterval {
		flushInterval = this.config.Stats.AlarmCountInterval
	}

	this.flusher = NewFlusher(this.config.Mongo, this.config.Flusher, flushInterval)
	this.alarmer = NewAlarmer(config.Alarm)
	this.clientProcessor = NewPipedClientProcessor(this.server, this.serverStats, this.flusher, this.alarmer)

	this.flusher.RegisterStats(this.clientProcessor.logProc.ElapsedStats)
	this.flusher.RegisterAlarmStats(this.clientProcessor.logProc.AlarmStats)

	return this
}

func (this *Piped) RunForever() {
	go server.StartPingServer(this.config.UdpPort)

	go this.server.LaunchTcpServer(this.config.ListenAddr, this.clientProcessor, this.config.SessionTimeout, 5)
	go this.serverStats.Start(this.config.StatsOutputInterval, this.config.MetricsLogfile)

	this.flusher.Serv()
	this.alarmer.Serv()

	done := make(chan int)
	<-done
}
