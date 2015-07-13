package analyser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nicholaskh/golib/db"
	log "github.com/nicholaskh/log4go"
	. "github.com/nicholaskh/piped/global"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

func (this *Analyser) loadElapsedStats(ts int64) {
	var result []interface{}
	err := db.MgoSession(this.mongoConfig.Addr).DB("ffan_monitor").C("sys_stats").Find(bson.M{"ts": ts}).Select(bson.M{"_id": 0}).All(&result)
	if err != nil && err != mgo.ErrNotFound {
		log.Error("load sys_stats error: %s", err.Error())
	}
	for _, vI := range result {
		v := vI.(bson.M)
		tag := v["tag"].(string)
		ts := v["ts"].(int64)
		value := v["value"]
		_, exists := this.ElapsedStats[tag]
		if !exists {
			this.ElapsedStats[tag] = make(map[int64]interface{})
		}
		this.ElapsedStats[tag][ts] = value
	}
}

func (this *Analyser) analysisElapsed(logLine string) {
	logPart := strings.Split(logLine, " ")
	var (
		elapsed float64
		uri     string
	)
	for _, part := range logPart {
		if uri == "" && strings.HasPrefix(part, "uri[") {
			uri = part[4 : len(part)-1]
			if uri = this.filterUri(uri); uri == "" {
				break
			}
		}
		if strings.HasPrefix(part, "ts[") {
			elapsed, _ = strconv.ParseFloat(part[3:len(part)-1], 64)
		}
	}
	if uri == "" || elapsed == 0 {
		return
	}
	minute := time.Now().Truncate(this.config.ElapsedCountInterval).Unix()
	tagElapsed := fmt.Sprintf("%s|%s", TAG_ELAPSED, uri)
	tagElapsedCount := fmt.Sprintf("%s_count|%s", TAG_ELAPSED, uri)
	this.elapsedStatsLock.Lock()
	if _, exists := this.ElapsedStats[tagElapsed]; !exists {
		this.ElapsedStats[tagElapsed] = make(map[int64]interface{})
	}
	oElapsed, exists := this.ElapsedStats[tagElapsed][minute]
	if !exists {
		oElapsed = float64(0)
	}
	if _, exists := this.ElapsedStats[tagElapsedCount]; !exists {
		this.ElapsedStats[tagElapsedCount] = make(map[int64]interface{})
	}
	elapsedCountCur, exists := this.ElapsedStats[tagElapsedCount][minute]
	if !exists {
		elapsedCountCur = 0
	}
	avgElapsed := (oElapsed.(float64)*float64(elapsedCountCur.(int)) + elapsed) / float64(elapsedCountCur.(int)+1)
	this.ElapsedStats[tagElapsedCount][minute] = elapsedCountCur.(int) + 1
	this.ElapsedStats[tagElapsed][minute] = avgElapsed
	this.elapsedStatsLock.Unlock()

	return
}

func (this *Analyser) filterUri(uri string) (uriFiltered string) {
	if strings.HasSuffix(uri, ".html") ||
		strings.HasSuffix(uri, ".png") ||
		strings.HasSuffix(uri, ".gif") ||
		strings.HasSuffix(uri, ".jpg") ||
		strings.HasSuffix(uri, ".js") ||
		strings.HasSuffix(uri, ".xml") ||
		strings.HasSuffix(uri, ".rar") ||
		strings.HasSuffix(uri, ".zip") ||
		strings.HasSuffix(uri, ".txt") ||
		strings.HasSuffix(uri, ".md5") ||
		strings.HasSuffix(uri, ".sql") ||
		strings.HasSuffix(uri, ".htaccess") {
		return ""
	}
	if this.config.ElapsedUriPathPrefix != nil {
		for _, prefix := range this.config.ElapsedUriPathPrefix {
			if strings.HasPrefix(uri, prefix) {
				uriFiltered = prefix
				return
			}
		}
	}
	fq := strings.Index(uri, "?")
	fsp := strings.Index(uri, "#")
	if (fq < fsp || fsp < 0) && fq > 0 {
		uri = uri[:fq]
	} else if (fsp < fq || fq < 0) && fsp > 0 {
		uri = uri[:fsp]
	}
	return uri
}
