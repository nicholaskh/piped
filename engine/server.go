package engine

import (
	"io"
	"net"
	"time"

	"github.com/nicholaskh/golib/server"
	log "github.com/nicholaskh/log4go"
	"github.com/nicholaskh/piped/config"
	"github.com/nicholaskh/piped/engine/alarmer"
	"github.com/nicholaskh/piped/engine/analyser"
	"github.com/nicholaskh/piped/engine/flusher"
)

type PipedClientProcessor struct {
	server      *server.TcpServer
	serverStats *ServerStats
	logProc     *LogProc
}

func NewPipedClientProcessor(server *server.TcpServer, serverStats *ServerStats, flusher *flusher.Flusher, alarmer *alarmer.Alarmer, analyser *analyser.Analyser) *PipedClientProcessor {
	this := new(PipedClientProcessor)
	this.server = server
	this.serverStats = serverStats
	this.logProc = NewLogProc(flusher, alarmer, analyser, config.PipedConf.Mongo)

	return this
}

func (this *PipedClientProcessor) OnAccept(c *server.Client) {
	proto := NewProtocol("")
	proto.SetConn(c.Conn)
	client := newClient(c, proto)
	for {
		if this.server.SessTimeout.Nanoseconds() > int64(0) {
			client.Proto.SetReadDeadline(time.Now().Add(this.server.SessTimeout))
		}

		app, data, err := client.proto.Read()

		if err != nil {
			err_, ok := err.(net.Error)
			if ok {
				if err_.Temporary() {
					log.Info("Temporary failure: %s", err_.Error())
					break
				}
			}
			if err == io.EOF {
				log.Info("Client %s closed the connection", client.Proto.RemoteAddr().String())
				break
			} else {
				log.Error(err.Error())
				break
			}
		}

		go this.OnRead(app, data)
	}
	client.Close()
}

func (this *PipedClientProcessor) OnRead(app, data []byte) {
	var (
		t1      time.Time
		elapsed time.Duration
	)

	t1 = time.Now()

	this.logProc.Process(app, data)

	elapsed = time.Since(t1)
	this.serverStats.CallLatencies.Update(elapsed.Nanoseconds() / 1e3)
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

type Client struct {
	*server.Client
	proto *Protocol
}

func newClient(c *server.Client, proto *Protocol) *Client {
	this := new(Client)
	this.Client = c
	this.proto = proto

	return this
}
