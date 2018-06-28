package croc

import (
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/frankenbeanies/uuid4"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type relayState struct {
	channel map[string]*channelData
	sync.RWMutex
}

var rs relayState

func init() {
	rs.Lock()
	rs.channel = make(map[string]*channelData)
	rs.Unlock()
}

func startServer(tcpPorts []string, port string) (err error) {
	// start cleanup on dangling channels
	go channelCleanup()

	// start server
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleWareHandler(), gin.Recovery())
	r.POST("/channel", func(c *gin.Context) {
		r, err := func(c *gin.Context) (r response, err error) {
			rs.Lock()
			defer rs.Unlock()
			r.Success = true
			var p payloadChannel
			err = c.ShouldBindJSON(&p)
			if err != nil {
				log.Errorf("failed on payload %+v", p)
				err = errors.Wrap(err, "problem parsing /channel")
				return
			}

			// determine if channel is invalid
			if _, ok := rs.channel[p.Channel]; !ok {
				err = errors.Errorf("channel '%s' does not exist", p.Channel)
				return
			}

			// determine if UUID is invalid for channel
			if p.UUID != rs.channel[p.Channel].uuids[0] &&
				p.UUID != rs.channel[p.Channel].uuids[1] {
				err = errors.Errorf("uuid '%s' is invalid", p.UUID)
				return
			}

			// check if the action is to close the channel
			if p.Close {
				delete(rs.channel, p.Channel)
				r.Message = "deleted " + p.Channel
				return
			}

			// assign each key provided
			assignedKeys := []string{}
			for key := range p.State {
				// TODO:
				// add a check that the value of key is not enormous

				// add only if it is a valid key
				if _, ok := rs.channel[p.Channel].State[key]; ok {
					assignedKeys = append(assignedKeys, key)
					rs.channel[p.Channel].State[key] = p.State[key]
				}
			}

			// return the current state
			r.Data = rs.channel[p.Channel]

			r.Message = fmt.Sprintf("assigned %d keys: %v", len(assignedKeys), assignedKeys)
			return
		}(c)
		if err != nil {
			log.Debugf("bad /channel: %s", err.Error())
			r.Message = err.Error()
			r.Success = false
		}
		bR, _ := json.Marshal(r)
		c.Data(200, "application/json", bR)
	})
	r.POST("/join", func(c *gin.Context) {
		r, err := func(c *gin.Context) (r response, err error) {
			rs.Lock()
			defer rs.Unlock()
			r.Success = true

			var p payloadOpen
			err = c.ShouldBindJSON(&p)
			if err != nil {
				log.Errorf("failed on payload %+v", p)
				err = errors.Wrap(err, "problem parsing")
				return
			}

			// determine if sender or recipient
			if p.Role != 0 && p.Role != 1 {
				err = errors.Errorf("no such role of %d", p.Role)
				return
			}

			// determine channel
			if p.Channel == "" {
				// TODO:
				// find an empty channel
				p.Channel = "chou"
			}
			if _, ok := rs.channel[p.Channel]; ok {
				// channel is not empty
				if rs.channel[p.Channel].uuids[p.Role] != "" {
					err = errors.Errorf("channel '%s' already occupied by role %d", p.Channel, p.Role)
					return
				}
			}
			r.Channel = p.Channel
			if _, ok := rs.channel[r.Channel]; !ok {
				rs.channel[r.Channel] = newChannelData(r.Channel)
			}

			// assign UUID for the role in the channel
			rs.channel[r.Channel].uuids[p.Role] = uuid4.New().String()
			r.UUID = rs.channel[r.Channel].uuids[p.Role]
			log.Debugf("(%s) %s has joined as role %d", r.Channel, r.UUID, p.Role)

			// if channel is not open, set initial parameters
			if !rs.channel[r.Channel].isopen {
				rs.channel[r.Channel].isopen = true
				rs.channel[r.Channel].Ports = tcpPorts
				rs.channel[r.Channel].startTime = time.Now()
				switch curve := p.Curve; curve {
				case "p224":
					rs.channel[r.Channel].curve = elliptic.P224()
				case "p256":
					rs.channel[r.Channel].curve = elliptic.P256()
				case "p384":
					rs.channel[r.Channel].curve = elliptic.P384()
				case "p521":
					rs.channel[r.Channel].curve = elliptic.P521()
				default:
					// TODO:
					// add SIEC
					p.Curve = "p256"
					rs.channel[r.Channel].curve = elliptic.P256()
				}
				log.Debugf("(%s) using curve '%s'", r.Channel, p.Curve)
				rs.channel[r.Channel].State["curve"] = []byte(p.Curve)
			}

			r.Message = fmt.Sprintf("assigned role %d in channel '%s'", p.Role, r.Channel)
			return
		}(c)
		if err != nil {
			log.Debugf("bad /join: %s", err.Error())
			r.Message = err.Error()
			r.Success = false
		}
		bR, _ := json.Marshal(r)
		c.Data(200, "application/json", bR)
	})
	log.Infof("Running at http://0.0.0.0:" + port)
	err = r.Run(":" + port)
	return
}

func middleWareHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()
		// Run next function
		c.Next()
		// Log request
		log.Infof("%v %v %v %s", c.Request.RemoteAddr, c.Request.Method, c.Request.URL, time.Since(t))
	}
}

func channelCleanup() {
	maximumWait := 10 * time.Minute
	for {
		rs.Lock()
		keys := make([]string, len(rs.channel))
		i := 0
		for key := range rs.channel {
			keys[i] = key
			i++
		}
		for _, key := range keys {
			if time.Since(rs.channel[key].startTime) > maximumWait {
				log.Debugf("channel %s has exceeded time, deleting", key)
				delete(rs.channel, key)
			}
		}
		rs.Unlock()
		time.Sleep(1 * time.Minute)
	}
}
