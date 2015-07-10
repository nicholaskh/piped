package config

import (
	"time"

	conf "github.com/nicholaskh/jsconf"
)

type AnalyserConfig struct {
	ElapsedUriPathPrefix []string

	StatsCountInterval   time.Duration
	ElapsedCountInterval time.Duration
	AlarmCountInterval   time.Duration
	XapiCountInterval    time.Duration

	MacThreshold   int
	PhoneThreshold int
}

func (this *AnalyserConfig) LoadConfig(cf *conf.Conf) {
	this.ElapsedUriPathPrefix = cf.StringList("elapsed_uri_path_prefix", nil)

	this.StatsCountInterval = cf.Duration("stats_count_interval", time.Hour)
	this.ElapsedCountInterval = cf.Duration("elapsed_count_interval", time.Minute*5)
	this.AlarmCountInterval = cf.Duration("alarm_count_interval", time.Minute)
	this.XapiCountInterval = cf.Duration("xapi_count_interval", time.Hour*24)

	this.MacThreshold = cf.Int("mac_threshold", 10)
	this.PhoneThreshold = cf.Int("phone_threshold", 10)
}
