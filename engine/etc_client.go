package engine

import (
	"fmt"
	"strings"

	"github.com/nicholaskh/etclib"
	"github.com/nicholaskh/golib/ip"
)

var localAddr string

func RegisterEtc(etcServers []string) error {
	err := etclib.Dial(etcServers)
	if err != nil {
		return err
	}
	err = etclib.BootService(localAddr, etclib.SERVICE_PIPED)
	return err
}

func UnregisterEtc() error {
	return etclib.ShutdownService(localAddr, etclib.SERVICE_PIPED)
}

func LoadLocalAddr(listenAddr string) {
	localIps := ip.LocalIpv4Addrs()
	if len(localIps) == 0 {
		panic("No local ip address found")
	}
	localAddr = localIps[0]

	listenPort := strings.Split(listenAddr, ":")[1]
	localAddr = fmt.Sprintf("%s:%s", localAddr, listenPort)
}
