package analyser

import (
	"strings"
	"sync"
	"time"

	"github.com/nicholaskh/piped/config"
	"github.com/nicholaskh/piped/engine/alarmer"
	"github.com/nicholaskh/piped/engine/flusher"
	. "github.com/nicholaskh/piped/global"
)

type Analyser struct {
	config *config.AnalyserConfig

	WebServerStats LogStats
	ElapsedStats   LogStats
	AlarmStats     LogStats
	XapiStats      LogStats

	statsMem     LogStats
	statsMemLock sync.Mutex

	alarmer *alarmer.Alarmer
	flusher *flusher.Flusher

	mongoConfig *config.MongoConfig

	dedup map[int64]map[string]int

	webServerStatsLock sync.Mutex
	elapsedStatsLock   sync.Mutex
	alarmStatsLock     sync.Mutex
	xapiStatsLock      sync.Mutex
	dedupLock          sync.Mutex

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

	this.WebServerStats = make(LogStats)
	this.ElapsedStats = make(LogStats)
	this.AlarmStats = make(LogStats)
	this.XapiStats = make(LogStats)

	this.flusher = flusher
	this.alarmer = alarmer

	this.dedup = make(map[int64]map[string]int)

	this.loadElapsedStats(time.Now().Truncate(this.config.ElapsedCountInterval).Unix())

	return this
}

func (this *Analyser) Enqueue(log *Log) {
	this.queue <- log
}

func (this *Analyser) Serv() {
	for {
		select {
		case logStruct := <-this.queue:
			switch logStruct.App {
			case "wifi":
				this.analysisWifiPortal(logStruct.LogLine)
			case "xapi":
				this.analysisXapi(logStruct)
			}

			switch logStruct.Tag {
			case TAG_APACHE_404, TAG_APACHE_500, TAG_NGINX_404, TAG_NGINX_500:
				this.analysisWebServer(logStruct)
			case TAG_APP:
				if strings.HasPrefix(logStruct.LogLine, "NOTICE") {
					this.analysisElapsed(logStruct.LogLine)
				}
			}
		}
	}
}
