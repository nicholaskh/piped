package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nicholaskh/golib/db"
	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

type LogStats map[string]map[int64]interface{}

type LogProc struct {
	Stats       LogStats
	flusher     *Flusher
	mongoConfig *config.MongoConfig
}

func NewLogProc(flusher *Flusher, mongoConfig *config.MongoConfig) *LogProc {
	this := new(LogProc)
	this.mongoConfig = mongoConfig
	this.Stats = make(LogStats)
	this.loadStats(time.Now().Unix() - time.Now().Unix()%STATS_COUNT_INTERVAL)
	this.flusher = flusher
	return this
}

func (this *LogProc) loadStats(ts int64) {
	var result []interface{}
	err := db.MgoSession(this.mongoConfig.Addr).DB("ffan_monitor").C("sys_stats").Find(bson.M{"ts": time.Now().Unix() - time.Now().Unix()%STATS_COUNT_INTERVAL}).Select(bson.M{"_id": 0}).All(&result)
	if err != nil && err != mgo.ErrNotFound {
		log.Error("load sys_stats error: %s", err.Error())
	}
	for _, vI := range result {
		v := vI.(bson.M)
		tag := v["tag"].(string)
		ts := v["ts"].(int64)
		value := v["value"]
		_, exists := this.Stats[tag]
		if !exists {
			this.Stats[tag] = make(map[int64]interface{})
		}
		this.Stats[tag][ts] = value
	}
}

func (this *LogProc) Process(input []byte) {
	line := string(input)
	log.Debug(line)
	linePart := strings.SplitN(line, LOG_SEP, 2)
	if len(linePart) < 2 {
		log.Error("Wrong format: %s", line)
	}
	tag := linePart[0]
	logg := linePart[1]

	switch tag {
	case TAG_APACHE_404, TAG_APACHE_500, TAG_NGINX_404, TAG_NGINX_500:
		t := time.Now().Unix()
		hr := t - t%int64(time.Hour/time.Second)
		_, exists := this.Stats[tag]
		if !exists {
			this.Stats[tag] = make(map[int64]interface{})
			this.Stats[tag][hr] = 0
		}
		var value int
		valueI, exists := this.Stats[tag][hr]
		if !exists {
			value = 0
		} else {
			value = valueI.(int)
		}
		this.Stats[tag][hr] = value + 1
	case TAG_APP:
		//store to the db
		if strings.HasPrefix(logg, "WARNING") || strings.HasPrefix(logg, "FATAL") {
			this.flusher.Enqueue(logg)
		}
		//doing statistic of elapsed
		if strings.HasPrefix(logg, "NOTICE") {
			//NOTICE: 15-05-15 14:51:53 errno[0] client[10.1.171.230] uri[/] user[] refer[http://10.1.169.16:12620/] cookie[U_UID=ced4bf452fea42b0853597fb6430e819; PHPSESSID=781f9621e47a41bbb15c4852f97c84af; SESSIONID=781f9621e47a41bbb15c4852f97c84af; CITY_ID=110100; PLAZA_ID=1000772] post[] ts[0.12319707870483]  f_redis[1]
			reg := regexp.MustCompile(`uri\[([^\?#\]]+)[^\]]*\].*ts\[([\d\.]+)\]`)
			logPart := reg.FindAllStringSubmatch(logg, -1)[0]
			if len(logPart) < 3 {
				log.Warn("elapsed log format error: %s", logg)
				break
			}
			uri := logPart[1]
			elapsed, _ := strconv.ParseFloat(logPart[2], 64)
			min := time.Now().Unix() - time.Now().Unix()%STATS_COUNT_INTERVAL
			tagElapsed := fmt.Sprintf("%s|%s", TAG_ELAPSED, uri)
			tagElapsedCount := fmt.Sprintf("%s_count|%s", TAG_ELAPSED, uri)
			if _, exists := this.Stats[tagElapsed]; !exists {
				this.Stats[tagElapsed] = make(map[int64]interface{})
			}
			oElapsed, exists := this.Stats[tagElapsed][min]
			if !exists {
				oElapsed = float64(0)
			}
			if _, exists := this.Stats[tagElapsedCount]; !exists {
				this.Stats[tagElapsedCount] = make(map[int64]interface{})
			}
			elapsedCountCur, exists := this.Stats[tagElapsedCount][min]
			if !exists {
				elapsedCountCur = 0
			}
			avgElapsed := (oElapsed.(float64) + elapsed) / float64(elapsedCountCur.(int)+1)
			this.Stats[tagElapsedCount][min] = elapsedCountCur.(int) + 1
			this.Stats[tagElapsed][min] = avgElapsed
		}
	}

	log.Debug(this.Stats)
	log.Debug("tag: %s", tag)
	log.Debug("log: %s", logg)
}
