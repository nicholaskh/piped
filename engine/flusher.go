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

func NewFlusher(mongoConfig *config.MongoConfig, statsFlushInterval time.Duration) *Flusher {
	this := new(Flusher)
	this.mongoConfig = mongoConfig
	this.statsFlushInterval = statsFlushInterval
	this.queue = make(chan string, 100000)

	return this
}

func (this *Flusher) RegisterStats(stats LogStats) {
	this.stats = stats
}

func (this *Flusher) Enqueue(logg string) {
	this.queue <- logg
}

func (this *Flusher) Serv() {
	for {
		select {
		case <-time.Tick(this.statsFlushInterval):
			log.Debug(this.stats)
			this.flushStats()
		case logg := <-this.queue:
			this.flushLog(logg)
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

func (this *Flusher) flushLog(logg string) {
	ts := time.Now().Unix()
	err := db.MgoSession(this.mongoConfig.Addr).DB("ffan_monitor").C("log").Insert(bson.M{"ts": ts, "log": logg})
	if err != nil {
		log.Error("flush stats error: %s", err.Error())
	}
}
