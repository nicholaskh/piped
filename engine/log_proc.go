package engine

import (
	"strings"
	"time"

	log "github.com/nicholaskh/log4go"
)

type LogStats map[string]map[int64]int

type LogProc struct {
	Stats LogStats
}

func NewLogProc() *LogProc {
	this := new(LogProc)
	this.Stats = make(LogStats)

	return this
}

func (this *LogProc) Process(input []byte) {
	line := string(input)
	linePart := strings.SplitN(line, LOG_SEP, 2)
	if len(linePart) < 2 {
		log.Error("Wrong format: %s", line)
	}
	tag := linePart[0]
	logg := linePart[1]

	switch tag {
	case TAG_APACHE_500:
		t := time.Now().Unix()
		hr := t - t%int64(time.Hour/time.Second)
		_, exists := this.Stats[tag]
		if !exists {
			this.Stats[tag] = make(map[int64]int)
			this.Stats[tag][hr] = 0
		}
		this.Stats[tag][hr]++
	}

	log.Info(this.Stats)

	log.Debug("tag: %s", tag)
	log.Debug("log: %s", logg)
}
