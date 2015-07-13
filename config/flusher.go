package config

import (
	"time"

	conf "github.com/nicholaskh/jsconf"
	. "github.com/nicholaskh/piped/global"
)

type FlusherConfig struct {
	StatsFlushInterval           time.Duration
	WifiPortalStatsFlushInterval time.Duration
	XapiStatsFlushInterval       time.Duration

	LogFlushType     int
	LogFlushInterval time.Duration
}

func (this *FlusherConfig) LoadConfig(cf *conf.Conf) {
	this.StatsFlushInterval = cf.Duration("stats_flush_interval", time.Second*5)
	this.WifiPortalStatsFlushInterval = cf.Duration("wifi_portal_stats_flush_interval", time.Minute)
	this.XapiStatsFlushInterval = cf.Duration("xapi_stats_flush_interval", time.Second*5)

	this.LogFlushType = cf.Int("log_flush_type", LOG_FLUSH_TYPE_INTERVAL)
	this.LogFlushInterval = cf.Duration("log_flush_interval", time.Second)
}
