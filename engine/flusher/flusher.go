package flusher

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
	mongoConfig     *config.MongoConfig
	elapsedStats    LogStats
	wifiPortalStats LogStats
	xapiStats       LogStats
	webServerStats  LogStats
	config          *config.FlusherConfig
	analyserConfig  *config.AnalyserConfig
	mongoPool       *db.MgoSessionPool

	queue chan *Log
}

func NewFlusher(mongoConfig *config.MongoConfig, flusherConfig *config.FlusherConfig, analyserConfig *config.AnalyserConfig) *Flusher {
	this := new(Flusher)
	this.mongoConfig = mongoConfig
	this.config = flusherConfig
	this.analyserConfig = analyserConfig
	this.queue = make(chan *Log, 100000)
	this.mongoPool = db.NewMgoSessionPool(this.mongoConfig.Addr, this.mongoConfig.Connections)

	return this
}

func (this *Flusher) RegisterStats(stats LogStats) {
	this.elapsedStats = stats
}

func (this *Flusher) RegisterWifiPortalStats(stats LogStats) {
	this.wifiPortalStats = stats
}

func (this *Flusher) RegisterXapiStats(stats LogStats) {
	this.xapiStats = stats
}

func (this *Flusher) RegisterWebServerStats(stats LogStats) {
	this.webServerStats = stats
}

func (this *Flusher) Enqueue(logg *Log) {
	this.queue <- logg
}

func (this *Flusher) Serv() {
	go this.servElapsed()
	go this.servWifiPortal()
	go this.servXapi()
	go this.servWebServer()

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

func (this *Flusher) servElapsed() {
	for {
		select {
		case <-time.Tick(this.config.StatsFlushInterval):
			this.flushStats(this.elapsedStats, this.config.StatsFlushInterval, this.analyserConfig.ElapsedCountInterval)
		}
	}
}

func (this *Flusher) servWifiPortal() {
	for {
		select {
		case <-time.Tick(this.config.WifiPortalStatsFlushInterval):
			this.flushStats(this.wifiPortalStats, this.config.WifiPortalStatsFlushInterval, this.analyserConfig.WifiPortalCountInterval)
		}
	}
}

func (this *Flusher) servXapi() {
	for {
		select {
		case <-time.Tick(this.config.XapiStatsFlushInterval):
			this.flushStats(this.xapiStats, this.config.XapiStatsFlushInterval, this.analyserConfig.XapiCountInterval)
		}
	}
}

func (this *Flusher) servWebServer() {
	for {
		select {
		case <-time.Tick(this.config.StatsFlushInterval):
			this.flushStats(this.webServerStats, this.config.StatsFlushInterval, this.analyserConfig.WebServerCountInterval)
		}
	}
}

func (this *Flusher) flushStats(stats LogStats, interval time.Duration, purgeTime time.Duration) {
	purgeTs := time.Now().Add(interval * -1).Truncate(purgeTime).Unix()

	mgoSession := this.mongoPool.Get()
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
		}
	}
	this.mongoPool.Put(mgoSession)
}

func (this *Flusher) flushLog(logStruct *Log) {
	ts := time.Now().Unix()
	mgoSession := this.mongoPool.Get()
	err := mgoSession.DB("ffan_monitor").C("log").Insert(bson.M{"ts": ts, "app": logStruct.App, "log": logStruct.LogLine})
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
			logStruct := <-this.queue
			records = append(records, bson.M{"ts": ts, "app": logStruct.App, "log": logStruct.LogLine})
		}
		mgoSession := this.mongoPool.Get()
		err := mgoSession.DB("ffan_monitor").C("log").Insert(records...)
		this.mongoPool.Put(mgoSession)
		if err != nil {
			log.Error("flush log error: %s", err.Error())
		}
	}
}
