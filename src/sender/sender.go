package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"

	log "github.com/cihub/seelog"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/schollz/croc/src/compress"
	"github.com/schollz/croc/src/crypt"
	"github.com/schollz/croc/src/logger"
	"github.com/schollz/croc/src/models"
	"github.com/schollz/croc/src/utils"
	"github.com/schollz/croc/src/zipper"
	"github.com/schollz/pake"
	"github.com/schollz/progressbar/v2"
	"github.com/tscholl2/siec"
)

var DebugLevel string

// Send is the async call to send data
func Send(done chan struct{}, c *websocket.Conn, fname string, codephrase string) {
	logger.SetLogLevel(DebugLevel)
	log.Debugf("sending %s", fname)
	err := send(c, fname, codephrase)
	if err != nil {
		if strings.HasPrefix(err.Error(), "websocket: close 100") {
			err = nil
		}
		if err == nil {
			fmt.Fprintf(os.Stderr, "Transfer complete")
		} else {
			fmt.Fprintf(os.Stderr, err.Error())
		}
	}
	done <- struct{}{}
}

func send(c *websocket.Conn, fname string, codephrase string) (err error) {
	// check that the file exists
	fname, err = filepath.Abs(fname)
	_, filename := filepath.Split(fname)
	f, err := os.Open(fname)
	if err != nil {
		return
	}
	fstat, err := f.Stat()
	if err != nil {
		return err
	}

	// get stats about the file
	fstats := models.FileStats{filename, fstat.Size(), fstat.ModTime(), fstat.IsDir(), fstat.Name()}
	if fstats.IsDir {
		// zip the directory
		fstats.SentName, err = zipper.ZipFile(fname, true)
		// remove the file when leaving
		defer os.Remove(fstats.SentName)
		fname = fstats.SentName
		if err != nil {
			return
		}
		f.Close()
		// reopen file
		f, err = os.Open(fstats.SentName)
		if err != nil {
			return
		}
		fstat, err := f.Stat()
		if err != nil {
			return err
		}
		// get new size
		fstats.Size = fstat.Size()
	}

	// get ready to generate session key
	var sessionKey []byte

	// start a spinner
	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	spin.Suffix = " wating for recipient..."
	spin.Start()

	// pick an elliptic curve
	curve := siec.SIEC255()
	// both parties should have a weak key
	pw := []byte(codephrase)
	// initialize sender P ("0" indicates sender)
	P, err := pake.Init(pw, 0, curve, 100*time.Millisecond)
	if err != nil {
		return
	}

	step := 0
	for {
		messageType, message, errRead := c.ReadMessage()
		if errRead != nil {
			return errRead
		}
		if messageType == websocket.PongMessage || messageType == websocket.PingMessage {
			continue
		}
		log.Debugf("got %d: %s", messageType, message)
		switch step {
		case 0:
			// send pake data
			log.Debugf("[%d] first, P sends u to Q", step)
			c.WriteMessage(websocket.BinaryMessage, P.Bytes())
			// start PAKE spinnner
			spin.Stop()
			spin.Suffix = " performing PAKE..."
			spin.Start()
		case 1:
			// P recieves H(k),v from Q
			log.Debugf("[%d] P computes k, H(k), sends H(k) to Q", step)
			if err := P.Update(message); err != nil {
				return err
			}
			c.WriteMessage(websocket.BinaryMessage, P.Bytes())
			sessionKey, _ = P.SessionKey()
			// check(err)
			log.Debugf("%x\n", sessionKey)
		case 2:
			log.Debugf("[%d] recipient declares readiness for file info", step)
			if !bytes.Equal(message, []byte("ready")) {
				return errors.New("recipient refused file")
			}
			fstatsBytes, err := json.Marshal(fstats)
			if err != nil {
				return err
			}
			log.Debugf("%s\n", fstatsBytes)
			c.WriteMessage(websocket.BinaryMessage, fstatsBytes)
		case 3:
			spin.Stop()

			log.Debugf("[%d] recipient declares readiness for file data", step)
			if !bytes.Equal(message, []byte("ready")) {
				return errors.New("recipient refused file")
			}

			fmt.Fprintf(os.Stderr, "Sending...\n")
			// send file, compure hash simultaneously
			buffer := make([]byte, 1024*512)
			bar := progressbar.NewOptions(
				int(fstats.Size),
				progressbar.OptionSetRenderBlankState(true),
				progressbar.OptionSetBytes(int(fstats.Size)),
				progressbar.OptionSetWriter(os.Stderr),
			)
			for {
				bytesread, err := f.Read(buffer)
				bar.Add(bytesread)
				if bytesread > 0 {
					// do compression
					compressedBytes := compress.Compress(buffer[:bytesread])
					// compressedBytes := buffer[:bytesread]

					// do encryption
					enc := crypt.Encrypt(compressedBytes, sessionKey, true)
					encBytes, err := json.Marshal(enc)
					if err != nil {
						return err
					}

					// send message
					err = c.WriteMessage(websocket.BinaryMessage, encBytes)
					if err != nil {
						err = errors.Wrap(err, "problem writing message")
						return err
					}
					// wait for ok
					c.ReadMessage()
				}
				if err != nil {
					if err != io.EOF {
						fmt.Println(err)
					}
					break
				}
			}

			bar.Finish()
			log.Debug("send hash to finish file")
			fileHash, err := utils.HashFile(fname)
			if err != nil {
				return err
			}
			c.WriteMessage(websocket.TextMessage, fileHash)
		case 4:
			log.Debugf("[%d] determing whether it went ok", step)
			if bytes.Equal(message, []byte("ok")) {
				log.Debug("file transfered successfully")
				return nil
			} else {
				return errors.New("file not transfered succesfully")
			}
		default:
			return fmt.Errorf("unknown step")
		}
		step++
	}
}
