package engine

import (
	"sync/atomic"
	"time"

	"github.com/nicholaskh/golib/db"
	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
	. "github.com/nicholaskh/piped/global"
	"labix.org/v2/mgo/bson"
)

type Flusher struct {
	mongoConfig *config.MongoConfig
	stats       LogStats
	alarmStats  LogStats
	config      *config.FlusherConfig
	purgeTime   time.Duration
	mongoPool   *db.MgoSessionPool

	queue chan *Log
}

func NewFlusher(mongoConfig *config.MongoConfig, flusherConfig *config.FlusherConfig, purgeTime time.Duration) *Flusher {
	this := new(Flusher)
	this.mongoConfig = mongoConfig
	this.config = flusherConfig
	this.purgeTime = purgeTime
	this.queue = make(chan *Log, 100000)
	this.mongoPool = db.NewMgoSessionPool(this.mongoConfig.Addr, this.mongoConfig.Connections)

	return this
}

func (this *Flusher) RegisterStats(stats LogStats) {
	this.stats = stats
}

func (this *Flusher) RegisterAlarmStats(stats LogStats) {
	this.alarmStats = stats
}

func (this *Flusher) Enqueue(logg *Log) {
	this.queue <- logg
}

func (this *Flusher) Serv() {
	go func() {
		for {
			select {
			case <-time.Tick(this.config.StatsFlushInterval):
				go this.flushStats(this.stats, this.config.StatsFlushInterval)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-time.Tick(this.config.AlarmStatsFlushInterval):
				go this.flushStats(this.alarmStats, this.config.AlarmStatsFlushInterval)
			}
		}
	}()

	go func() {
		switch this.config.LogFlushType {
		case LOG_FLUSH_TYPE_EACH:
			for {
				select {
				case logg := <-this.queue:
					this.flushLog(logg)
				}
			}
		case LOG_FLUSH_TYPE_INTERVAL:
			for {
				var i int32
				select {
				case <-time.Tick(this.config.LogFlushInterval):
					if atomic.CompareAndSwapInt32(&i, 0, 1) {
						this.flushLogBatch()
						i = 0
					}
				}
			}
		}
	}()
}

func (this *Flusher) flushStats(stats LogStats, interval time.Duration) {
	purgeTs := time.Now().Add(interval * -2).Truncate(interval).Unix()

	mgoSession := this.mongoPool.Get()
	log.Info(stats)
	for tag, stats := range stats {
		for ts, value := range stats {
			if ts < purgeTs {
				delete(stats, ts)
			} else {
				_, err := mgoSession.DB("ffan_monitor").C("sys_stats").Upsert(bson.M{"tag": tag, "ts": ts}, bson.M{"tag": tag, "ts": ts, "value": value})
				if err != nil {
					log.Error("flush stats error: %s", err.Error())
				}
			}
			log.Info("save stats[%d]%s", ts, value)
		}
	}
	this.mongoPool.Put(mgoSession)
}

func (this *Flusher) flushLog(logg *Log) {
	ts := time.Now().Unix()
	mgoSession := this.mongoPool.Get()
	err := mgoSession.DB("ffan_monitor").C("log").Insert(bson.M{"ts": ts, "app": logg.app, "log": logg.data})
	this.mongoPool.Put(mgoSession)
	if err != nil {
		log.Error("flush stats error: %s", err.Error())
	}
}

func (this *Flusher) flushLogBatch() {
	logCount := len(this.queue)
	if logCount > 0 {
		records := make([]interface{}, 0)
		// TODO ts should be the time when the log was collected, if log_flush_interval is too long, ts may be wrong
		ts := time.Now().Unix()
		for i := 0; i < logCount; i++ {
			logg := <-this.queue
			records = append(records, bson.M{"ts": ts, "app": logg.app, "log": logg.data})
		}
		mgoSession := this.mongoPool.Get()
		err := mgoSession.DB("ffan_monitor").C("log").Insert(records...)
		this.mongoPool.Put(mgoSession)
		if err != nil {
			log.Error("flush log error: %s", err.Error())
		}
	}
}
