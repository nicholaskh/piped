package engine

import (
	"errors"
	"strings"
	"sync"

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

	mongoConfig *config.MongoConfig

	elapsedStatsLock sync.Mutex
	alarmStatsLock   sync.Mutex
	xapiStatsLock    sync.Mutex
}

func NewLogProc(flusher *flusher.Flusher, alarmer *alarmer.Alarmer, analyser *analyser.Analyser, mongoConfig *config.MongoConfig) *LogProc {
	this := new(LogProc)
	this.mongoConfig = mongoConfig

	this.flusher = flusher
	this.analyser = analyser

	return this
}

func (this *LogProc) Process(app, data []byte) error {
	line := string(data)
	linePart := strings.SplitN(line, LOG_SEP, 2)
	if len(linePart) < 2 {
		log.Error("Wrong format: %s", line)
		return errors.New("Wrong format")
	}
	tag := linePart[0]
	logg := linePart[1]

	logStruct := &Log{string(app), tag, logg}
	this.analyser.Enqueue(logStruct)

	if tag == TAG_APP ||
		tag == TAG_MEMBER_ACTIVITY ||
		tag == TAG_MEMBER_ACTIVITY_COUPON ||
		tag == TAG_MEMBER_COUPON {
		this.flusher.Enqueue(logStruct)
	}
	return nil
}
