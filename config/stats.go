package config

import (
	"time"

	conf "github.com/nicholaskh/jsconf"
)

type StatsConfig struct {
	StatsCountInterval   time.Duration
	ElapsedCountInterval time.Duration
}

func (this *StatsConfig) LoadConfig(cf *conf.Conf) {
	this.StatsCountInterval = cf.Duration("stats_count_interval", time.Hour)
	this.ElapsedCountInterval = cf.Duration("elapsed_count_interval", time.Minute*5)
}
