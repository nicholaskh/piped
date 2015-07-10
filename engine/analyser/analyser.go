package analyser

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nicholaskh/golib/db"
	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
	"github.com/nicholaskh/piped/engine/alarmer"
	"github.com/nicholaskh/piped/engine/flusher"
	. "github.com/nicholaskh/piped/global"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

type Analyser struct {
	config *config.AnalyserConfig

	ElapsedStats LogStats
	AlarmStats   LogStats
	XapiStats    LogStats

	statsMem     LogStats
	statsMemLock sync.Mutex

	alarmer *alarmer.Alarmer
	flusher *flusher.Flusher

	mongoConfig *config.MongoConfig

	dedup map[int64]map[string]int

	elapsedStatsLock sync.Mutex
	alarmStatsLock   sync.Mutex
	xapiStatsLock    sync.Mutex

	emailSentTimes map[string]time.Time
	smsSentTimes   map[string]time.Time

	queue chan *Log
}

func NewAnalyser(config *config.AnalyserConfig, mongoConfig *config.MongoConfig, flusher *flusher.Flusher, alarmer *alarmer.Alarmer) *Analyser {
	this := new(Analyser)
	this.config = config
	this.mongoConfig = mongoConfig

	this.queue = make(chan *Log, ANALYSER_BACKLOG)

	this.statsMem = make(LogStats)

	this.ElapsedStats = make(LogStats)
	this.AlarmStats = make(LogStats)
	this.XapiStats = make(LogStats)

	this.flusher = flusher
	this.alarmer = alarmer

	this.loadStats(time.Now().Truncate(this.config.ElapsedCountInterval).Unix())

	return this
}

func (this *Analyser) loadStats(ts int64) {
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

func (this *Analyser) Enqueue(log *Log) {
	this.queue <- log
}

func (this *Analyser) Serv() {
	for {
		select {
		case logStruct := <-this.queue:
			var (
				uri string
				err error
			)
			switch logStruct.App {
			case "wifi":
				//				err = this.analysisWifiPortal(logStruct.LogLine)
				if err != nil {
					break
				}
			case "xapi":
				//				err = this.analysisXapi(logStruct.LogLine)
				if err != nil {
					break
				}
			}

			switch logStruct.Tag {
			case TAG_APACHE_404, TAG_APACHE_500, TAG_NGINX_404, TAG_NGINX_500:
				hr := time.Now().Truncate(this.config.StatsCountInterval).Unix()
				_, exists := this.ElapsedStats[logStruct.Tag]
				if !exists {
					this.ElapsedStats[logStruct.Tag] = make(map[int64]interface{})
					this.ElapsedStats[logStruct.Tag][hr] = 0
				}
				var value int
				valueI, exists := this.ElapsedStats[logStruct.Tag][hr]
				if !exists {
					value = 0
				} else {
					value = valueI.(int)
				}
				this.ElapsedStats[logStruct.Tag][hr] = value + 1
			case TAG_APP:
				//store to the db
				if strings.HasPrefix(logStruct.LogLine, "NOTICE") {
					//doing statistic of elapsed
					logPart := strings.Split(logStruct.LogLine, " ")
					var elapsed float64
					count := 0
					for _, part := range logPart {
						if uri == "" && strings.HasPrefix(part, "uri[") {
							uri = part[4 : len(part)-1]
							if uri = this.filterUri(uri); uri == "" {
								break
							}
							count++
							if count >= 2 {
								break
							}
						}
						if strings.HasPrefix(part, "ts[") {
							elapsed, _ = strconv.ParseFloat(part[3:len(part)-1], 64)
							count++
							if count >= 2 {
								break
							}
						}
					}
					if uri == "" || elapsed == 0 {
						break
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
				}
			}
		}
	}
}

func (this *Analyser) analysisWifiPortal(logLine string) error {
	var uri string
	logPart := strings.Split(logLine, " ")
	var mac, phone string
	for _, part := range logPart {
		if uri == "" && strings.HasPrefix(part, "uri[") {
			uri = part[4 : len(part)-1]
			v, err := url.ParseRequestURI(uri)
			if err != nil {
				log.Error("Parse uri error: %s", err.Error())
				return err
			}
			q, _ := url.ParseQuery(v.RawQuery)
			var exists bool
			var queryArr []string
			if queryArr, exists = q["CMAC"]; exists && len(queryArr) > 0 {
				mac = queryArr[0]
			}
			if queryArr, exists = q["mobile"]; exists && len(queryArr) > 0 {
				phone = queryArr[0]
			}
			if uri = this.filterUri(uri); uri == "" {
				break
			}
			break
		}
	}
	if uri == "" || mac == "" && phone == "" {
		return errors.New("No info in log for wifi portal stats")
	}
	truncateTime := time.Now().Truncate(this.config.AlarmCountInterval)
	minute := truncateTime.Unix()
	if mac != "" {
		tag := fmt.Sprintf("%s|%s", uri, mac)
		this.statsMemLock.Lock()
		if _, exists := this.statsMem[tag]; !exists {
			this.statsMem[tag] = make(map[int64]interface{})
		}
		ct, exists := this.statsMem[tag][minute]
		if !exists {
			ct = 0
		}
		currentCount := ct.(int) + 1
		this.statsMem[tag][minute] = currentCount
		this.statsMemLock.Unlock()
		if currentCount >= this.config.MacThreshold {
			this.alarmStatsLock.Lock()
			if _, exists := this.AlarmStats[tag]; !exists {
				this.AlarmStats[tag] = make(map[int64]interface{})
			}
			this.AlarmStats[tag][minute] = currentCount
			this.alarmStatsLock.Unlock()
			this.enqueueEmailAlarm("mac", mac, truncateTime.Format("2006-01-02 15:04:05"), currentCount)
			this.enqueueSmsAlarm("mac", mac, truncateTime.Format("2006-01-02 15:04:05"), currentCount)
		}
	}
	if phone != "" {
		tag := fmt.Sprintf("%s|%s", uri, phone)
		this.statsMemLock.Lock()
		if _, exists := this.statsMem[tag]; !exists {
			this.statsMem[tag] = make(map[int64]interface{})
		}
		ct, exists := this.statsMem[tag][minute]
		if !exists {
			ct = 0
		}
		currentCount := ct.(int) + 1
		this.statsMem[tag][minute] = currentCount
		this.statsMemLock.Unlock()
		if currentCount >= this.config.PhoneThreshold {
			this.alarmStatsLock.Lock()
			if _, exists := this.AlarmStats[tag]; !exists {
				this.AlarmStats[tag] = make(map[int64]interface{})
			}
			this.AlarmStats[tag][minute] = currentCount
			this.alarmStatsLock.Unlock()
			this.enqueueEmailAlarm("phone", phone, truncateTime.Format("2006-01-02 15:04:05"), currentCount)
			this.enqueueSmsAlarm("phone", phone, truncateTime.Format("2006-01-02 15:04:05"), currentCount)
		}
	}

	return nil
}

func (this *Analyser) analysisXapi(logLine string) error {
	logPart := strings.Split(logLine, " ")
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

	tag := fmt.Sprintf("xapi-total")
	this.xapiStatsLock.Lock()
	if _, exists := this.XapiStats[tag]; !exists {
		this.XapiStats[tag] = make(map[int64]interface{})
	}
	ct, exists := this.XapiStats[tag][truncateTs]
	if !exists {
		ct = 0
	}
	currentCount := ct.(int) + 1
	this.XapiStats[tag][truncateTs] = currentCount
	this.xapiStatsLock.Unlock()

	if mobile != "" {
		this.countXapiDup(tag, mobile, truncateTs)
	}

	if isRegister == "" || couponLevel == "" || responseSt == "" {
		return errors.New("No info in log for xapi stats")
	}

	if isRegister != "" {
		this.countXapiStats("isRegister", isRegister, truncateTs)
	}
	if couponLevel == "L1" || couponLevel == "L2" {
		this.countXapiStats("couponLevel", couponLevel, truncateTs)
		if mobile != "" {
			this.countXapiDup(tag, mobile, truncateTs)
		}
	}
	if responseSt != "" {
		this.countXapiStats("response-st", responseSt, truncateTs)

		if mobile != "" {
			this.countXapiDup(tag, mobile, truncateTs)
		}
	}

	return nil
}

func (this *Analyser) countXapiStats(statsKey, statsVal string, truncateTs int64) {
	tag := fmt.Sprintf("%s|%s", statsKey, statsVal)
	this.xapiStatsLock.Lock()
	if _, exists := this.XapiStats[tag]; !exists {
		this.XapiStats[tag] = make(map[int64]interface{})
	}
	ct, exists := this.XapiStats[tag][truncateTs]
	if !exists {
		ct = 0
	}
	currentCount := ct.(int) + 1
	this.XapiStats[tag][truncateTs] = currentCount
	this.xapiStatsLock.Unlock()
}

func (this *Analyser) countXapiDup(tag, mobile string, truncateTs int64) {
	dedupTag := fmt.Sprintf("%s|%s", tag, mobile)
	_, exists := this.dedup[truncateTs]
	if !exists {
		this.dedup[truncateTs] = make(map[string]int)
	}
	_, exists = this.dedup[truncateTs][dedupTag]
	if !exists {
		this.dedup[truncateTs][dedupTag] = 1
	}
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

func (this *Analyser) enqueueSmsAlarm(limitType, id, timeStr string, count int) {
	if t, exists := this.smsSentTimes[id]; !exists || t.Add(this.alarmer.Config.Sms.SendInterval).Before(time.Now()) {
		this.alarmer.EnqueueSms(alarmer.NewSms(fmt.Sprintf("Request times exceed, %s: %s, time: %s, request times: %d",
			limitType, id, timeStr, count)))
		this.smsSentTimes[id] = time.Now()
	}
}

func (this *Analyser) enqueueEmailAlarm(limitType, id, timeStr string, count int) {
	if t, exists := this.emailSentTimes[id]; !exists || t.Add(this.alarmer.Config.Email.SendInterval).Before(time.Now()) {
		this.alarmer.EnqueueEmail(alarmer.NewEmail("【ALARM】Request times exceed",
			this.constructEmailBody(limitType, id, timeStr, count)))
		this.emailSentTimes[id] = time.Now()
	}
}

func (this *Analyser) constructEmailBody(tp, addr, time string, times int) string {
	return fmt.Sprintf(`
		<html>
		<body>
		<h3>
		Request Times exceed
		</h3>
		<p>%s: %s</p>
		<p>time: %s</p>
		<p>request times: %d</p>
		<br />
		If you do not care about this message, please ignore.
		</body>
		</html>
		`, tp, addr, time, times)
}
