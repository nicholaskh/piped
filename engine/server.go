package engine

import (
	//	"encoding/json"
	"io"
	"net"
	//	"strings"
	"time"

	"github.com/nicholaskh/golib/server"
	log "github.com/nicholaskh/log4go"
)

type PipedClientProcessor struct {
	server      *server.TcpServer
	serverStats *ServerStats
	logProc     *LogProc
}

func NewPipedClientProcessor(server *server.TcpServer, serverStats *ServerStats) *PipedClientProcessor {
	this := new(PipedClientProcessor)
	this.server = server
	this.serverStats = serverStats
	this.logProc = NewLogProc()

	return this
}

func (this *PipedClientProcessor) OnAccept(client *server.Client) {
	for {
		if this.server.SessTimeout.Nanoseconds() > int64(0) {
			client.SetReadDeadline(time.Now().Add(this.server.SessTimeout))
		}

		input := make([]byte, 1460)
		n, err := client.Conn.Read(input)
		input = input[:n]

		if err != nil {
			err_, ok := err.(net.Error)
			if ok {
				if err_.Temporary() {
					log.Info("Temporary failure: %s", err_.Error())
					continue
				}
			}
			if err == io.EOF {
				log.Info("Client %s closed the connection", client.RemoteAddr().String())
				break
			} else {
				log.Error(err.Error())
				break
			}
		}

		this.OnRead(client, input)
	}
	client.Close()
}

func (this *PipedClientProcessor) OnRead(client *server.Client, input []byte) {
	var (
		t1      time.Time
		elapsed time.Duration
	)

	t1 = time.Now()

	this.logProc.Process(input)

	elapsed = time.Since(t1)
	this.serverStats.CallLatencies.Update(elapsed.Nanoseconds() / 1e6)
	this.serverStats.CallPerSecond.Mark(1)

}

type TinyFluentRecord struct {
	Timestamp uint64
	Data      map[string]interface{}
}

type FluentRecordSet struct {
	Tag     string
	Records []TinyFluentRecord
}
