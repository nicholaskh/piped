package config

import (
	"time"

	conf "github.com/nicholaskh/jsconf"
)

var (
	PipedConf *PipedConfig
)

type PipedConfig struct {
	EtcServers []string

	ListenAddr     string
	SessionTimeout time.Duration

	StatsListenAddr string
	ProfListenAddr  string

	MetricsLogfile      string
	StatsOutputInterval time.Duration

	UdpPort int

	Mongo    *MongoConfig
	Flusher  *FlusherConfig
	Alarm    *AlarmConfig
	Analyser *AnalyserConfig
}

func (this *PipedConfig) LoadConfig(cf *conf.Conf) {
	this.EtcServers = cf.StringList("etc_servers", nil)
	if this.EtcServers == nil {
		panic("No etc servers found")
	}

	this.ListenAddr = cf.String("listen_addr", ":5687")
	this.SessionTimeout = cf.Duration("session_timeout", time.Minute*2)

	this.StatsListenAddr = cf.String("stats_listen_addr", ":9030")
	this.ProfListenAddr = cf.String("prof_listen_addr", ":9031")

	this.MetricsLogfile = cf.String("metrics_logfile", "metrics.log")
	this.StatsOutputInterval = cf.Duration("stats_output_interval", time.Minute*10)

	this.UdpPort = cf.Int("udp_port", 14570)

	this.Mongo = new(MongoConfig)
	section, err := cf.Section("mongodb")
	if err != nil {
		panic("Mongodb config not found")
	}
	this.Mongo.LoadConfig(section)

	this.Flusher = new(FlusherConfig)
	section, err = cf.Section("flusher")
	if err != nil {
		panic("Flusher config not found")
	}
	this.Flusher.LoadConfig(section)

	this.Alarm = new(AlarmConfig)
	section, err = cf.Section("alarm")
	if err != nil {
		panic("Alarm config not found")
	}
	this.Alarm.LoadConfig(section)

	this.Analyser = new(AnalyserConfig)
	section, err = cf.Section("analyser")
	if err != nil {
		panic("Analyser config not found")
	}
	this.Analyser.LoadConfig(section)
}
