package analyser

import (
	"fmt"
	"strings"
	"time"

	. "github.com/nicholaskh/piped/global"
)

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

	if isRegister == "" || couponLevel == "" || responseSt == "" {
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
	dedupTag := fmt.Sprintf("%s|%s", tag, mobile)
	_, exists := this.dedup[truncateTs]
	if !exists {
		this.dedup[truncateTs] = make(map[string]int)
	}
	_, exists = this.dedup[truncateTs][dedupTag]
	if exists {
		return
	}
	this.dedup[truncateTs][dedupTag] = 1
	this.dedupLock.Unlock()

	this.countXapiStats(dedupTag, truncateTs)
}
