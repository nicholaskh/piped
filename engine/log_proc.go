package engine

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nicholaskh/golib/db"
	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
	. "github.com/nicholaskh/piped/global"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

type LogStats map[string]map[int64]interface{}

type LogProc struct {
	config       *config.StatsConfig
	Stats        LogStats
	statsMem     LogStats
	statsMemLock sync.Mutex
	flusher      *Flusher
	alarmer      *Alarmer
	mongoConfig  *config.MongoConfig
	appReg       *regexp.Regexp
	statsLock    sync.Mutex

	emailSentTimes map[string]time.Time
	smsSentTimes   map[string]time.Time
}

func NewLogProc(config *config.StatsConfig, flusher *Flusher, alarmer *Alarmer, mongoConfig *config.MongoConfig) *LogProc {
	this := new(LogProc)
	this.config = config
	this.mongoConfig = mongoConfig
	this.Stats = make(LogStats)
	this.statsMem = make(LogStats)
	this.loadStats(time.Now().Truncate(this.config.ElapsedCountInterval).Unix())
	this.flusher = flusher
	this.alarmer = alarmer
	//NOTICE: 15-05-15 14:51:53 errno[0] client[10.1.171.230] uri[/] user[] refer[http://10.1.169.16:12620/] cookie[U_UID=ced4bf452fea42b0853597fb6430e819; PHPSESSID=781f9621e47a41bbb15c4852f97c84af; SESSIONID=781f9621e47a41bbb15c4852f97c84af; CITY_ID=110100; PLAZA_ID=1000772] post[] ts[0.12319707870483]  f_redis[1]
	this.appReg = regexp.MustCompile(`uri\[([^\?#\]]+)[^\]]*\].*ts\[([\d\.]+)\]`)

	this.emailSentTimes = make(map[string]time.Time)
	this.smsSentTimes = make(map[string]time.Time)
	return this
}

func (this *LogProc) loadStats(ts int64) {
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
		_, exists := this.Stats[tag]
		if !exists {
			this.Stats[tag] = make(map[int64]interface{})
		}
		this.Stats[tag][ts] = value
	}
}

func (this *LogProc) Process(app, data []byte) {
	var uri string
	appStr := string(app)
	line := string(data)
	linePart := strings.SplitN(line, LOG_SEP, 2)
	if len(linePart) < 2 {
		log.Error("Wrong format: %s", line)
		return
	}
	tag := linePart[0]
	logg := linePart[1]

	//"wifi" app should count the request times and alarm
	if appStr == "wifi" {
		logPart := strings.Split(logg, " ")
		var mac, phone string
		for _, part := range logPart {
			if uri == "" && strings.HasPrefix(part, "uri[") {
				uri = part[4 : len(part)-1]
				v, err := url.ParseRequestURI(uri)
				if err != nil {
					log.Error("Parse uri error: %s", err.Error())
					return
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
			log.Warn("wifi log format error: %s", logg)
			return
		}
		minuteTime := time.Now().Truncate(this.config.AlarmCountInterval)
		minute := minuteTime.Unix()
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
				this.statsLock.Lock()
				if _, exists := this.Stats[tag]; !exists {
					this.Stats[tag] = make(map[int64]interface{})
				}
				this.Stats[tag][minute] = currentCount
				this.statsLock.Unlock()
				this.enqueueEmailAlarm("mac", mac, minuteTime.Format("2006-01-02 15:04:05"), currentCount)
				this.enqueueSmsAlarm("mac", mac, minuteTime.Format("2006-01-02 15:04:05"), currentCount)
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
				this.statsLock.Lock()
				if _, exists := this.Stats[tag]; !exists {
					this.Stats[tag] = make(map[int64]interface{})
				}
				this.Stats[tag][minute] = currentCount
				this.statsLock.Unlock()
				this.enqueueEmailAlarm("phone", phone, minuteTime.Format("2006-01-02 15:04:05"), currentCount)
				this.enqueueSmsAlarm("phone", phone, minuteTime.Format("2006-01-02 15:04:05"), currentCount)
			}
		}
	}

	switch tag {
	case TAG_APACHE_404, TAG_APACHE_500, TAG_NGINX_404, TAG_NGINX_500:
		hr := time.Now().Truncate(this.config.StatsCountInterval).Unix()
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
		if strings.HasPrefix(logg, "NOTICE") {
			//doing statistic of elapsed
			/**
			subMatch := this.appReg.FindAllStringSubmatch(logg, -1)
			if len(subMatch) < 1 || len(subMatch[0]) < 3 {
				log.Warn("elapsed log format error: %s", logg)
				break
			}
			uri := subMatch[0][1]
			if uri = this.filterUri(uri); uri == "" {
				return
			}
			elapsed, _ := strconv.ParseFloat(subMatch[0][2], 64)
			*/
			logPart := strings.Split(logg, " ")
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
				log.Warn("elapsed log format error: %s", logg)
				break
			}
			minute := time.Now().Truncate(this.config.ElapsedCountInterval).Unix()
			tagElapsed := fmt.Sprintf("%s|%s", TAG_ELAPSED, uri)
			tagElapsedCount := fmt.Sprintf("%s_count|%s", TAG_ELAPSED, uri)
			this.statsLock.Lock()
			if _, exists := this.Stats[tagElapsed]; !exists {
				this.Stats[tagElapsed] = make(map[int64]interface{})
			}
			oElapsed, exists := this.Stats[tagElapsed][minute]
			if !exists {
				oElapsed = float64(0)
			}
			if _, exists := this.Stats[tagElapsedCount]; !exists {
				this.Stats[tagElapsedCount] = make(map[int64]interface{})
			}
			elapsedCountCur, exists := this.Stats[tagElapsedCount][minute]
			if !exists {
				elapsedCountCur = 0
			}
			avgElapsed := (oElapsed.(float64)*float64(elapsedCountCur.(int)) + elapsed) / float64(elapsedCountCur.(int)+1)
			this.Stats[tagElapsedCount][minute] = elapsedCountCur.(int) + 1
			this.Stats[tagElapsed][minute] = avgElapsed
			this.statsLock.Unlock()
			this.flusher.Enqueue(&Log{appStr, logg})
		} else if strings.HasPrefix(logg, "WARNING") || strings.HasPrefix(logg, "FATAL") {
			this.flusher.Enqueue(&Log{appStr, logg})
		}
	}
}

func (this *LogProc) filterUri(uri string) (uriFiltered string) {
	if strings.HasSuffix(uri, ".html") ||
		strings.HasSuffix(uri, ".png") ||
		strings.HasSuffix(uri, ".gif") ||
		strings.HasSuffix(uri, ".jpg") ||
		strings.HasSuffix(uri, ".js") ||
		strings.HasSuffix(uri, ".xml") ||
		strings.HasSuffix(uri, ".rar") ||
		strings.HasSuffix(uri, ".zip") {
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

func (this *LogProc) constructEmailBody(tp, addr, time string, times int) string {
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

func (this *LogProc) enqueueSmsAlarm(limitType, id, timeStr string, count int) {
	if t, exists := this.smsSentTimes[id]; !exists || t.Add(this.alarmer.config.Sms.SendInterval).Before(time.Now()) {
		this.alarmer.EnqueueSms(NewSms(fmt.Sprintf("Request times exceed, %s: %s, time: %s, request times: %d",
			limitType, id, timeStr, count)))
		this.smsSentTimes[id] = time.Now()
	}
}

func (this *LogProc) enqueueEmailAlarm(limitType, id, timeStr string, count int) {
	if t, exists := this.emailSentTimes[id]; !exists || t.Add(this.alarmer.config.Email.SendInterval).Before(time.Now()) {
		this.alarmer.EnqueueEmail(NewEmail("【ALARM】Request times exceed",
			this.constructEmailBody(limitType, id, timeStr, count)))
		this.emailSentTimes[id] = time.Now()
	}
}

type Log struct {
	app  string
	data string
}
