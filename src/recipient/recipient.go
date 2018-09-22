package recipient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	log "github.com/cihub/seelog"
	"github.com/gorilla/websocket"
	"github.com/schollz/croc/src/compress"
	"github.com/schollz/croc/src/crypt"
	"github.com/schollz/croc/src/logger"
	"github.com/schollz/croc/src/models"
	"github.com/schollz/croc/src/utils"
	"github.com/schollz/pake"
	"github.com/schollz/progressbar"
	"github.com/tscholl2/siec"
)

var DebugLevel string

// Receive is the async operation to receive a file
func Receive(done chan struct{}, c *websocket.Conn) {
	logger.SetLogLevel(DebugLevel)

	err := receive(c)
	if err != nil {
		log.Error(err)
	}
	done <- struct{}{}
}

func receive(c *websocket.Conn) (err error) {
	var fstats models.FileStats
	var sessionKey []byte

	// pick an elliptic curve
	curve := siec.SIEC255()
	// both parties should have a weak key
	pw := []byte{1, 2, 3}

	// initialize recipient Q ("1" indicates recipient)
	Q, err := pake.Init(pw, 1, curve, 100*time.Millisecond)
	if err != nil {
		return
	}

	step := 0
	for {
		messageType, message, err := c.ReadMessage()
		if err != nil {
			return err
		}
		if messageType == websocket.PongMessage || messageType == websocket.PingMessage {
			continue
		}

		log.Debugf("got %d: %s", messageType, message)
		switch step {
		case 0:
			// Q receives u
			log.Debugf("[%d] Q computes k, sends H(k), v back to P", step)
			if err := Q.Update(message); err != nil {
				return err
			}
			c.WriteMessage(websocket.BinaryMessage, Q.Bytes())
		case 1:
			log.Debugf("[%d] Q recieves H(k) from P", step)
			if err := Q.Update(message); err != nil {
				return err
			}

			sessionKey, err = Q.SessionKey()
			if err != nil {
				return err
			}
			log.Debugf("%x\n", sessionKey)
			c.WriteMessage(websocket.BinaryMessage, []byte("ready"))
		case 2:
			log.Debugf("[%d] recieve file info", step)
			err = json.Unmarshal(message, &fstats)
			if err != nil {
				return err
			}
			// await file
			f, err := os.Create("out")
			if err != nil {
				return err
			}
			bytesWritten := 0
			bar := progressbar.NewOptions(
				int(fstats.Size),
				progressbar.OptionSetRenderBlankState(true),
				progressbar.OptionSetBytes(int(fstats.Size)),
			)
			c.WriteMessage(websocket.BinaryMessage, []byte("ready"))
			for {
				messageType, message, err := c.ReadMessage()
				if err != nil {
					return err
				}
				if messageType == websocket.PongMessage || messageType == websocket.PingMessage {
					continue
				}
				if messageType == websocket.BinaryMessage {
					// tell the sender that we recieved this packet
					c.WriteMessage(websocket.BinaryMessage, []byte("ok"))

					// do decryption
					var enc crypt.Encryption
					err = json.Unmarshal(message, &enc)
					if err != nil {
						return err
					}
					decrypted, err := enc.Decrypt(sessionKey, true)
					if err != nil {
						return err
					}

					// do decompression
					decompressed := compress.Decompress(decrypted)
					// decompressed := decrypted

					// write to file
					n, err := f.Write(decompressed)
					if err != nil {
						return err
					}
					// update the bytes written
					bytesWritten += n
					// update the progress bar
					bar.Add(n)
				} else {
					// we are finished

					// close file
					err = f.Close()
					if err != nil {
						return err
					}

					// finish bar
					bar.Finish()

					// check hash
					hash256, err := utils.HashFile("out")
					if err != nil {
						return err
					}

					// check success hash(myfile) == hash(theirfile)
					log.Debugf("got hash: %x", message)
					if bytes.Equal(hash256, message) {
						c.WriteMessage(websocket.BinaryMessage, []byte("ok"))
						return nil
					} else {
						c.WriteMessage(websocket.BinaryMessage, []byte("not"))
						return errors.New("file corrupted")
					}
				}
			}
		default:
			return fmt.Errorf("unknown step")
		}
		step++
	}
}
