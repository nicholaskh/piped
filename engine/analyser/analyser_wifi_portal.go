package analyser

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/engine/alarmer"
)

func (this *Analyser) analysisWifiPortal(logLine string) {
	var uri string
	logPart := strings.Split(logLine, " ")
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
		return
	}
	truncateTime := time.Now().Truncate(this.config.WifiPortalCountInterval)
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

	return
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
