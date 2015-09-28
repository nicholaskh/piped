package analyser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/engine/alarmer"
	. "github.com/nicholaskh/piped/global"
)

type ResponseJson struct {
	Data   interface{} `json:"data"`
	Status int         `json:"status"`
}

func (this *Analyser) analysisGift(logStruct *Log) {
	logPart := strings.Split(logStruct.LogLine, " ")
	var status int
	var data interface{}
	var uid, mobile string

	for _, part := range logPart {
		if strings.HasPrefix(part, "response[") {
			response := part[9 : len(part)-1]
			status, data = this.parseResponse(response)
		}

		if strings.HasPrefix(part, "url[") {
			urlString := part[4 : len(part)-1]
			v, err := url.ParseRequestURI(urlString)
			if err != nil {
				log.Warn(err)
			}
			u := v.RawQuery
			url.ParseQuery(u)
			queryMapping := v.Query()
			uid = queryMapping["uid"][0]
			mobile = queryMapping["mobile"][0]
		}
	}

	log.Debug("uid: %s, mobile: %s, status: %d, data: %x", uid, mobile, status, data)
	if status != 200 || data == nil {
		this.alarmer.EnqueueEmail(alarmer.NewEmail("【ALARM】礼品派发失败",
			fmt.Sprintf("uid[%s], status[%d], mobile[%s], req_time[%s %s]", uid, status, mobile, logPart[1], logPart[2])))
	}
}

func (this *Analyser) parseResponse(response string) (status int, data interface{}) {
	var r ResponseJson
	err := json.Unmarshal([]byte(response), &r)
	if err != nil {
		log.Error(err.Error())
	}
	status = r.Status
	data = r.Data
	return
}
