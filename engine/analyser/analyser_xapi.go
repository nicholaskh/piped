package analyser

import (
	"fmt"
	"strings"
	"time"

	"github.com/nicholaskh/golib/db"
	log "github.com/nicholaskh/log4go"
	. "github.com/nicholaskh/piped/global"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

func (this *Analyser) loadXapiStats(ts int64) {
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
		if strings.HasPrefix(tag, "ma|") {
			_, exists := this.XapiStats[tag]
			if !exists {
				this.XapiStats[tag] = make(map[int64]interface{})
			}
			this.XapiStats[tag][ts] = value
		}
	}
}

func (this *Analyser) analysisXapi(logStruct *Log) {
	logPart := strings.Split(logStruct.LogLine, " ")
	var isRegister, couponLevel, mobile, responseSt string

	truncateTime := time.Now().Truncate(this.config.XapiCountInterval)
	truncateTs := truncateTime.Unix()

	for _, part := range logPart {
		if strings.HasPrefix(part, "isRegister[") {
			isRegister = part[11 : len(part)-1]
		}

		if strings.HasPrefix(part, "couponLevel[") {
			couponLevel = part[12 : len(part)-1]
		}

		if strings.HasPrefix(part, "mobile[") {
			mobile = part[7 : len(part)-1]
		}

		if strings.HasPrefix(part, "response-st[") {
			responseSt = part[12 : len(part)-1]
		}
	}

	this.countXapiStats(fmt.Sprintf("%s|%s", logStruct.Tag, "xapi-total"), truncateTs)

	if mobile != "" {
		this.countXapiDedup(fmt.Sprintf("%s|%s", logStruct.Tag, "xapi-total"), mobile, truncateTs)
	}

	if isRegister == "" && couponLevel == "" && responseSt == "" {
		return
	}

	if isRegister == "0" {
		this.countXapiStats(fmt.Sprintf("%s|%s|%s", logStruct.Tag, "isRegister", isRegister), truncateTs)
	}
	if couponLevel == "L1" || couponLevel == "L2" {
		tag := fmt.Sprintf("%s|%s|%s", logStruct.Tag, "couponLevel", couponLevel)
		this.countXapiStats(tag, truncateTs)
		if mobile != "" {
			this.countXapiDedup(tag, mobile, truncateTs)
		}
	}
	if responseSt != "" {
		tag := fmt.Sprintf("%s|%s|%s", logStruct.Tag, "response-st", responseSt)
		this.countXapiStats(tag, truncateTs)

		if mobile != "" {
			this.countXapiDedup(tag, mobile, truncateTs)
		}
	}

	return
}

func (this *Analyser) countXapiStats(tag string, truncateTs int64) {
	this.xapiStatsLock.Lock()
	if _, exists := this.XapiStats[tag]; !exists {
		this.XapiStats[tag] = make(map[int64]interface{})
	}
	ct, exists := this.XapiStats[tag][truncateTs]
	if !exists {
		ct = 0
	}
	this.XapiStats[tag][truncateTs] = ct.(int) + 1
	this.xapiStatsLock.Unlock()
}

func (this *Analyser) countXapiDedup(tag, mobile string, truncateTs int64) {
	this.dedupLock.Lock()
	dupTag := fmt.Sprintf("%s|%s", tag, mobile)
	dedupTag := fmt.Sprintf("%s|dedup", tag)
	_, exists := this.dedup[truncateTs]
	if !exists {
		this.dedup[truncateTs] = make(map[string]int)
	}
	_, exists = this.dedup[truncateTs][dupTag]
	if exists {
		this.dedupLock.Unlock()
		return
	}
	this.dedup[truncateTs][dupTag] = 1
	this.dedupLock.Unlock()

	this.countXapiStats(dedupTag, truncateTs)
}

func (this *Analyser) clearExpiredDupRecord(currHour time.Time) {
	for t, _ := range this.dedup {
		if t < currHour.Unix() {
			delete(this.dedup, t)
		}
	}
}
