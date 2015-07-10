package engine

import (
	"strings"
	"sync"
	"time"

	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
	"github.com/nicholaskh/piped/engine/alarmer"
	"github.com/nicholaskh/piped/engine/analyser"
	"github.com/nicholaskh/piped/engine/flusher"
	. "github.com/nicholaskh/piped/global"
)

type LogProc struct {
	flusher  *flusher.Flusher
	analyser *analyser.Analyser
	alarmer  *alarmer.Alarmer

	mongoConfig *config.MongoConfig

	dedup map[int64]map[string]int

	elapsedStatsLock sync.Mutex
	alarmStatsLock   sync.Mutex
	xapiStatsLock    sync.Mutex

	emailSentTimes map[string]time.Time
	smsSentTimes   map[string]time.Time
}

func NewLogProc(flusher *flusher.Flusher, alarmer *alarmer.Alarmer, analyser *analyser.Analyser, mongoConfig *config.MongoConfig) *LogProc {
	this := new(LogProc)
	this.mongoConfig = mongoConfig

	this.flusher = flusher
	this.analyser = analyser

	this.dedup = make(map[int64]map[string]int)

	this.emailSentTimes = make(map[string]time.Time)
	this.smsSentTimes = make(map[string]time.Time)
	return this
}

func (this *LogProc) Process(app, data []byte) {
	line := string(data)
	linePart := strings.SplitN(line, LOG_SEP, 2)
	if len(linePart) < 2 {
		log.Error("Wrong format: %s", line)
		return
	}
	tag := linePart[0]
	logg := linePart[1]

	logStruct := &Log{string(app), tag, logg}
	this.analyser.Enqueue(logStruct)

	if tag == TAG_APP {
		this.flusher.Enqueue(logStruct)
	}
}
