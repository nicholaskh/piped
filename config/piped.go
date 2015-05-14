package config

import (
	"time"

	conf "github.com/nicholaskh/jsconf"
)

var (
	PipedConf *PipedConfig
)

type PipedConfig struct {
	ListenAddr     string
	SessionTimeout time.Duration

	StatsListenAddr string
	ProfListenAddr  string

	MetricsLogfile      string
	StatsOutputInterval time.Duration

	StatsFlushInterval time.Duration

	Mongo *MongoConfig
}

func (this *PipedConfig) LoadConfig(cf *conf.Conf) {
	this.ListenAddr = cf.String("listen_addr", ":5687")
	this.SessionTimeout = cf.Duration("session_timeout", time.Minute*2)

	this.StatsListenAddr = cf.String("stats_listen_addr", ":9030")
	this.ProfListenAddr = cf.String("prof_listen_addr", ":9031")

	this.MetricsLogfile = cf.String("metrics_logfile", "metrics.log")
	this.StatsOutputInterval = cf.Duration("stats_output_interval", time.Minute*10)

	this.StatsFlushInterval = cf.Duration("stats_flush_interval", time.Second*10)

	this.Mongo = new(MongoConfig)
	section, err := cf.Section("mongodb")
	if err != nil {
		panic("Mongodb config not found")
	}
	this.Mongo.LoadConfig(section)
}
