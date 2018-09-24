package croc

import (
	"time"

	"github.com/schollz/croc/src/logger"
	"github.com/schollz/croc/src/recipient"
	"github.com/schollz/croc/src/relay"
	"github.com/schollz/croc/src/sender"
	"github.com/schollz/croc/src/zipper"
)

// Croc options
type Croc struct {
	// Options for all
	Debug bool
	// ShowText will display text on the stderr
	ShowText bool

	// Options for relay
	RelayWebsocketPort string
	RelayTCPPort       string
	CurveType          string

	// Options for connecting to server
	Address              string
	AddressTCPPort       string
	AddressWebsocketPort string
	Timeout              time.Duration
	LocalOnly            bool
	NoLocal              bool

	// Options for file transfering
	UseEncryption       bool
	UseCompression      bool
	AllowLocalDiscovery bool
	NoRecipientPrompt   bool
	Stdout              bool
	ForceSend           int // 0: ignore, 1: websockets, 2: TCP

	// Parameters for file transfer
	Filename   string
	Codephrase string

	// localIP address
	localIP string
	// is using local relay
	isLocal      bool
	normalFinish bool
}

// Init will initiate with the default parameters
func Init(debug bool) (c *Croc) {
	c = new(Croc)
	c.UseCompression = true
	c.UseEncryption = true
	c.AllowLocalDiscovery = true
	c.RelayWebsocketPort = "8153"
	c.RelayTCPPort = "8154"
	c.CurveType = "siec"
	c.Address = "198.199.67.130"
	c.AddressWebsocketPort = "8153"
	c.AddressTCPPort = "8154"
	c.NoRecipientPrompt = true
	debugLevel := "info"
	if debug {
		debugLevel = "debug"
		c.Debug = true
	}
	logger.SetLogLevel(debugLevel)
	sender.DebugLevel = debugLevel
	recipient.DebugLevel = debugLevel
	relay.DebugLevel = debugLevel
	zipper.DebugLevel = debugLevel
	return
}
