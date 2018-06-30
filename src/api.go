package croc

import (
	"net"
	"time"

	log "github.com/cihub/seelog"
	"github.com/schollz/peerdiscovery"
)

func init() {
	SetLogLevel("debug")
}

// Relay initiates a relay
func (c *Croc) Relay() error {
	// start relay
	go c.startRelay()

	// start server
	return c.startServer()
}

// Send will take an existing file or folder and send it through the croc relay
func (c *Croc) Send(fname string, codephrase string) (err error) {
	// start relay for listening
	runClientError := make(chan error)
	go func() {
		d := Init()
		d.ServerPort = "8140"
		d.TcpPorts = []string{"27140", "27141"}
		go d.startRelay()
		go d.startServer()
		e := Init()
		e.WebsocketAddress = "ws://127.0.0.1:8140"
		runClientError <- e.client(0, codephrase, fname)
	}()

	// start main client
	go func() {
		runClientError <- c.client(0, codephrase, fname)
	}()
	return <-runClientError
}

// Receive will receive something through the croc relay
func (c *Croc) Receive(codephrase string) (err error) {
	// try to discovery codephrase and server through peer network
	discovered, errDiscover := peerdiscovery.Discover(peerdiscovery.Settings{
		Limit:     1,
		TimeLimit: 1 * time.Second,
		Delay:     50 * time.Millisecond,
		Payload:   []byte(codephrase),
	})
	if errDiscover != nil {
		log.Debug(errDiscover)
	}
	if len(discovered) > 0 {
		log.Debugf("discovered %s on %s", discovered[0].Payload, discovered[0].Address)
		_, connectTimeout := net.DialTimeout("tcp", discovered[0].Address+":27140", 1*time.Second)
		if connectTimeout == nil {
			log.Debug("connected")
			c.WebsocketAddress = "ws://" + discovered[0].Address + ":8140"
			log.Debug(discovered[0].Address)
			codephrase = string(discovered[0].Payload)
		} else {
			log.Debug("but could not connect to ports")
		}
	} else {
		log.Debug("discovered no peers")
	}

	return c.client(1, codephrase)
}
