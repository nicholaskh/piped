package config

import (
	"time"

	conf "github.com/nicholaskh/jsconf"
)

type StatsConfig struct {
	StatsCountInterval   time.Duration
	ElapsedCountInterval time.Duration
	AlarmCountInterval   time.Duration

	ElapsedUriPathPrefix []string

	MacThreshold   int
	PhoneThreshold int
}

func (this *StatsConfig) LoadConfig(cf *conf.Conf) {
	this.StatsCountInterval = cf.Duration("stats_count_interval", time.Hour)
	this.ElapsedCountInterval = cf.Duration("elapsed_count_interval", time.Minute*5)
	this.AlarmCountInterval = cf.Duration("alarm_count_interval", time.Minute)

	this.ElapsedUriPathPrefix = cf.StringList("elapsed_uri_path_prefix", nil)

	this.MacThreshold = cf.Int("mac_threshold", 10)
	this.PhoneThreshold = cf.Int("phone_threshold", 10)
}
