package engine

import (
	"time"

	"github.com/nicholaskh/golib/db"
	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
	"labix.org/v2/mgo/bson"
)

type Flusher struct {
	mongoConfig        *config.MongoConfig
	stats              LogStats
	statsFlushInterval time.Duration
	queue              chan string
}

func NewFlusher(mongoConfig *config.MongoConfig, stats LogStats, statsFlushInterval time.Duration) *Flusher {
	this := new(Flusher)
	this.mongoConfig = mongoConfig
	this.stats = stats
	this.statsFlushInterval = statsFlushInterval

	return this
}

func (this *Flusher) Serv() {
	for {
		select {
		case <-time.Tick(this.statsFlushInterval):
			log.Info(this.stats)
			this.flushStats()
		}
	}
}

func (this *Flusher) flushStats() {
	for tag, stats := range this.stats {
		for ts, count := range stats {
			_, err := db.MgoSession(this.mongoConfig.Addr).DB("ffan_monitor").C("sys_stats").Upsert(bson.M{"tag": tag, "ts": ts}, bson.M{"tag": tag, "ts": ts, "count": count})
			if err != nil {
				log.Error("flush stats error: %s", err.Error())
			}
		}
	}
}
