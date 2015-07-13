package analyser

import (
	"time"

	. "github.com/nicholaskh/piped/global"
)

func (this *Analyser) analysisWebServer(logStruct *Log) {
	hr := time.Now().Truncate(this.config.WebServerCountInterval).Unix()
	this.webServerStatsLock.Lock()
	_, exists := this.WebServerStats[logStruct.Tag]
	if !exists {
		this.WebServerStats[logStruct.Tag] = make(map[int64]interface{})
		this.WebServerStats[logStruct.Tag][hr] = 0
	}
	var value int
	valueI, exists := this.WebServerStats[logStruct.Tag][hr]
	if !exists {
		value = 0
	} else {
		value = valueI.(int)
	}
	this.WebServerStats[logStruct.Tag][hr] = value + 1
	this.webServerStatsLock.Unlock()

	return
}
