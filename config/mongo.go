package config

import (
	"time"

	conf "github.com/nicholaskh/jsconf"
)

type MongoConfig struct {
	Addr          string
	Connections   int
	ConnTimeout   time.Duration
	SyncTimeout   time.Duration
	SocketTimeout time.Duration
}

func (this *MongoConfig) LoadConfig(cf *conf.Conf) {
	this.Addr = cf.String("addr", ":27017")
	this.Connections = cf.Int("connections", 3)
	this.ConnTimeout = cf.Duration("conn_timeout", time.Second*5)
	this.SyncTimeout = cf.Duration("sync_timeout", time.Second*3)
	this.SocketTimeout = cf.Duration("socket_timeout", this.SyncTimeout)
}
