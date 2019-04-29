package croc

import (
	"bytes"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/denisbrodbeck/machineid"
	"github.com/schollz/croc/v6/src/comm"
	"github.com/schollz/croc/v6/src/crypt"
	"github.com/schollz/croc/v6/src/logger"
	"github.com/schollz/croc/v6/src/utils"
	"github.com/schollz/pake"
	"github.com/schollz/progressbar/v2"
	"github.com/schollz/spinner"
)

const BufferSize = 4096 * 10
const Channels = 1

func init() {
	logger.SetLogLevel("debug")
}

func Debug(debug bool) {
	if debug {
		logger.SetLogLevel("debug")
	} else {
		logger.SetLogLevel("warn")
	}
}

type Options struct {
	IsSender     bool
	SharedSecret string
	Debug        bool
	AddressRelay string
	Stdout       bool
	NoPrompt     bool
}

type Client struct {
	Options Options
	Pake    *pake.Pake

	// steps involved in forming relationship
	Step1ChannelSecured       bool
	Step2FileInfoTransfered   bool
	Step3RecipientRequestFile bool
	Step4FileTransfer         bool
	Step5CloseChannels        bool

	// send / receive information of all files
	FilesToTransfer           []FileInfo
	FilesToTransferCurrentNum int

	// send / receive information of current file
	CurrentFile       *os.File
	CurrentFileChunks []int64

	// tcp connectios
	conn [17]*comm.Comm

	bar       *progressbar.ProgressBar
	spinner   *spinner.Spinner
	machineID string

	mutex *sync.Mutex
	quit  chan bool
}

type Message struct {
	Type    string `json:"t,omitempty"`
	Message string `json:"m,omitempty"`
	Bytes   []byte `json:"b,omitempty"`
	Num     int    `json:"n,omitempty"`
}

type Chunk struct {
	Bytes    []byte `json:"b,omitempty"`
	Location int64  `json:"l,omitempty"`
}

type FileInfo struct {
	Name         string    `json:"n,omitempty"`
	FolderRemote string    `json:"fr,omitempty"`
	FolderSource string    `json:"fs,omitempty"`
	Hash         []byte    `json:"h,omitempty"`
	Size         int64     `json:"s,omitempty"`
	ModTime      time.Time `json:"m,omitempty"`
	IsCompressed bool      `json:"c,omitempty"`
	IsEncrypted  bool      `json:"e,omitempty"`
}

type RemoteFileRequest struct {
	CurrentFileChunks         []int64
	FilesToTransferCurrentNum int
}

type SenderInfo struct {
	MachineID       string
	FilesToTransfer []FileInfo
}

func (m Message) String() string {
	b, _ := json.Marshal(m)
	return string(b)
}

// New establishes a new connection for transfering files between two instances.
func New(ops Options) (c *Client, err error) {
	c = new(Client)

	// setup basic info
	c.Options = ops
	Debug(c.Options.Debug)
	log.Debugf("options: %+v", c.Options)

	// initialize pake
	if c.Options.IsSender {
		c.Pake, err = pake.Init([]byte(c.Options.SharedSecret), 1, elliptic.P521(), 1*time.Microsecond)
	} else {
		c.Pake, err = pake.Init([]byte(c.Options.SharedSecret), 0, elliptic.P521(), 1*time.Microsecond)
	}
	if err != nil {
		return
	}

	c.mutex = &sync.Mutex{}
	return
}

type TransferOptions struct {
	PathToFiles      []string
	KeepPathInRemote bool
}

// Send will send the specified file
func (c *Client) Send(options TransferOptions) (err error) {
	return c.transfer(options)
}

// Receive will receive a file
func (c *Client) Receive() (err error) {
	return c.transfer(TransferOptions{})
}

func (c *Client) transfer(options TransferOptions) (err error) {
	if c.Options.IsSender {
		c.FilesToTransfer = make([]FileInfo, len(options.PathToFiles))
		totalFilesSize := int64(0)
		for i, pathToFile := range options.PathToFiles {
			var fstats os.FileInfo
			var fullPath string
			fullPath, err = filepath.Abs(pathToFile)
			if err != nil {
				return
			}
			fullPath = filepath.Clean(fullPath)
			var folderName string
			folderName, _ = filepath.Split(fullPath)

			fstats, err = os.Stat(fullPath)
			if err != nil {
				return
			}
			c.FilesToTransfer[i] = FileInfo{
				Name:         fstats.Name(),
				FolderRemote: ".",
				FolderSource: folderName,
				Size:         fstats.Size(),
				ModTime:      fstats.ModTime(),
			}
			c.FilesToTransfer[i].Hash, err = utils.HashFile(fullPath)
			totalFilesSize += fstats.Size()
			if err != nil {
				return
			}
			if options.KeepPathInRemote {
				var curFolder string
				curFolder, err = os.Getwd()
				if err != nil {
					return
				}
				curFolder, err = filepath.Abs(curFolder)
				if err != nil {
					return
				}
				if !strings.HasPrefix(folderName, curFolder) {
					err = fmt.Errorf("remote directory must be relative to current")
					return
				}
				c.FilesToTransfer[i].FolderRemote = strings.TrimPrefix(folderName, curFolder)
				c.FilesToTransfer[i].FolderRemote = filepath.ToSlash(c.FilesToTransfer[i].FolderRemote)
				c.FilesToTransfer[i].FolderRemote = strings.TrimPrefix(c.FilesToTransfer[i].FolderRemote, "/")
				if c.FilesToTransfer[i].FolderRemote == "" {
					c.FilesToTransfer[i].FolderRemote = "."
				}
			}
			log.Debugf("file %d info: %+v", i, c.FilesToTransfer[i])
		}
		fname := fmt.Sprintf("%d files", len(c.FilesToTransfer))
		if len(c.FilesToTransfer) == 1 {
			fname = fmt.Sprintf("'%s'", c.FilesToTransfer[0].Name)
		}
		machID, macIDerr := machineid.ID()
		if macIDerr != nil {
			log.Error(macIDerr)
			return
		}
		if len(machID) > 6 {
			machID = machID[:6]
		}
		c.machineID = machID
		fmt.Fprintf(os.Stderr, "Sending %s (%s) from your machine, '%s'\n", fname, utils.ByteCountDecimal(totalFilesSize), machID)
		fmt.Fprintf(os.Stderr, "Code is: %s\nOn the other computer run\n\ncroc %s\n", c.Options.SharedSecret, c.Options.SharedSecret)
		c.spinner.Suffix = " waiting for recipient..."
	}
	c.spinner.Start()
	// create channel for quitting
	// quit with c.quit <- true
	c.quit = make(chan bool)

	// if recipient, initialize with sending pake information
	log.Debug("ready")
	if !c.Options.IsSender && !c.Step1ChannelSecured {
		err = c.redisdb.Publish(c.nameOutChannel, Message{
			Type:  "pake",
			Bytes: c.Pake.Bytes(),
		}.String()).Err()
		if err != nil {
			return
		}
	}

	// listen for incoming messages and process them
	for {
		select {
		case <-c.quit:
			return
		case msg := <-c.incomingMessageChannel:
			var m Message
			err = json.Unmarshal([]byte(msg.Payload), &m)
			if err != nil {
				return
			}
			if m.Type == "finished" {
				err = c.redisdb.Publish(c.nameOutChannel, Message{
					Type: "finished",
				}.String()).Err()
				return err
			}
			err = c.processMessage(m)
			if err != nil {
				return
			}
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}
	return
}

func (c *Client) processMessage(m Message) (err error) {
	switch m.Type {
	case "pake":
		if c.spinner.Suffix != " performing PAKE..." {
			c.spinner.Stop()
			c.spinner.Suffix = " performing PAKE..."
			c.spinner.Start()
		}
		notVerified := !c.Pake.IsVerified()
		err = c.Pake.Update(m.Bytes)
		if err != nil {
			return
		}
		if (notVerified && c.Pake.IsVerified() && !c.Options.IsSender) || !c.Pake.IsVerified() {
			err = c.redisdb.Publish(c.nameOutChannel, Message{
				Type:  "pake",
				Bytes: c.Pake.Bytes(),
			}.String()).Err()
		}
		if c.Pake.IsVerified() {
			log.Debug(c.Pake.SessionKey())
			c.Step1ChannelSecured = true
		}
	case "error":
		c.spinner.Stop()
		fmt.Print("\r")
		err = fmt.Errorf("peer error: %s", m.Message)
		return err
	case "fileinfo":
		var senderInfo SenderInfo
		var decryptedBytes []byte
		key, _ := c.Pake.SessionKey()
		decryptedBytes, err = crypt.DecryptFromBytes(m.Bytes, key)
		if err != nil {
			log.Error(err)
			return
		}
		err = json.Unmarshal(decryptedBytes, &senderInfo)
		if err != nil {
			log.Error(err)
			return
		}
		c.FilesToTransfer = senderInfo.FilesToTransfer
		fname := fmt.Sprintf("%d files", len(c.FilesToTransfer))
		if len(c.FilesToTransfer) == 1 {
			fname = fmt.Sprintf("'%s'", c.FilesToTransfer[0].Name)
		}
		totalSize := int64(0)
		for _, fi := range c.FilesToTransfer {
			totalSize += fi.Size
		}
		c.spinner.Stop()
		if !c.Options.NoPrompt {
			fmt.Fprintf(os.Stderr, "\rAccept %s (%s) from machine '%s'? (y/n) ", fname, utils.ByteCountDecimal(totalSize), senderInfo.MachineID)
			if strings.ToLower(strings.TrimSpace(utils.GetInput(""))) != "y" {
				err = c.redisdb.Publish(c.nameOutChannel, Message{
					Type:    "error",
					Message: "refusing files",
				}.String()).Err()
				return fmt.Errorf("refused files")
			}
		} else {
			fmt.Fprintf(os.Stderr, "\rReceiving %s (%s) from machine '%s'\n", fname, utils.ByteCountDecimal(totalSize), senderInfo.MachineID)
		}
		log.Debug(c.FilesToTransfer)
		c.Step2FileInfoTransfered = true
	case "recipientready":
		var remoteFile RemoteFileRequest
		var decryptedBytes []byte
		key, _ := c.Pake.SessionKey()
		decryptedBytes, err = crypt.DecryptFromBytes(m.Bytes, key)
		if err != nil {
			log.Error(err)
			return
		}
		err = json.Unmarshal(decryptedBytes, &remoteFile)
		if err != nil {
			return
		}
		c.FilesToTransferCurrentNum = remoteFile.FilesToTransferCurrentNum
		c.CurrentFileChunks = remoteFile.CurrentFileChunks
		c.Step3RecipientRequestFile = true
	case "datachannel-offer":
		err = c.dataChannelReceive()
		if err != nil {
			return
		}
		err = c.recvSess.SetSDP(m.Message)
		if err != nil {
			return
		}
		var answer string
		answer, err = c.recvSess.CreateAnswer()
		if err != nil {
			return
		}
		// Output the answer in base64 so we can paste it in browser
		err = c.redisdb.Publish(c.nameOutChannel, Message{
			Type:    "datachannel-answer",
			Message: answer,
			Num:     m.Num,
		}.String()).Err()
		// start receiving data
		pathToFile := path.Join(c.FilesToTransfer[c.FilesToTransferCurrentNum].FolderRemote, c.FilesToTransfer[c.FilesToTransferCurrentNum].Name)
		c.spinner.Stop()
		key, _ := c.Pake.SessionKey()
		c.recvSess.ReceiveData(pathToFile, c.FilesToTransfer[c.FilesToTransferCurrentNum].Size, key)
		log.Debug("sending close-sender")
		err = c.redisdb.Publish(c.nameOutChannel, Message{
			Type: "close-sender",
		}.String()).Err()
	case "datachannel-answer":
		log.Debug("got answer:", m.Message)
		// Apply the answer as the remote description
		err = c.sendSess.SetSDP(m.Message)
		pathToFile := path.Join(c.FilesToTransfer[c.FilesToTransferCurrentNum].FolderSource, c.FilesToTransfer[c.FilesToTransferCurrentNum].Name)
		c.spinner.Stop()
		fmt.Fprintf(os.Stderr, "\r\nTransfering...\n")
		key, _ := c.Pake.SessionKey()
		c.sendSess.TransferFile(pathToFile, key)
	case "close-sender":
		log.Debug("close-sender received...")
		c.Step4FileTransfer = false
		c.Step3RecipientRequestFile = false
		c.sendSess.StopSending()
		log.Debug("sending close-recipient")
		err = c.redisdb.Publish(c.nameOutChannel, Message{
			Type: "close-recipient",
			Num:  m.Num,
		}.String()).Err()
	case "close-recipient":
		c.Step4FileTransfer = false
		c.Step3RecipientRequestFile = false
	}
	if err != nil {
		return
	}
	err = c.updateState()

	return
}

func (c *Client) updateState() (err error) {
	if c.Options.IsSender && c.Step1ChannelSecured && !c.Step2FileInfoTransfered {
		var b []byte
		b, err = json.Marshal(SenderInfo{
			MachineID:       c.machineID,
			FilesToTransfer: c.FilesToTransfer,
		})
		if err != nil {
			log.Error(err)
			return
		}
		key, _ := c.Pake.SessionKey()
		err = c.redisdb.Publish(c.nameOutChannel, Message{
			Type:  "fileinfo",
			Bytes: crypt.EncryptToBytes(b, key),
		}.String()).Err()
		if err != nil {
			return
		}
		c.Step2FileInfoTransfered = true
	}
	if !c.Options.IsSender && c.Step2FileInfoTransfered && !c.Step3RecipientRequestFile {
		// find the next file to transfer and send that number
		// if the files are the same size, then look for missing chunks
		finished := true
		for i, fileInfo := range c.FilesToTransfer {
			if i < c.FilesToTransferCurrentNum {
				continue
			}
			fileHash, errHash := utils.HashFile(path.Join(fileInfo.FolderRemote, fileInfo.Name))
			if errHash != nil || !bytes.Equal(fileHash, fileInfo.Hash) {
				if !bytes.Equal(fileHash, fileInfo.Hash) {
					log.Debugf("hashes are not equal %x != %x", fileHash, fileInfo.Hash)
				}
				finished = false
				c.FilesToTransferCurrentNum = i
				break
			}
			// TODO: print out something about this file already existing
		}
		if finished {
			// TODO: do the last finishing stuff
			log.Debug("finished")
			err = c.redisdb.Publish(c.nameOutChannel, Message{
				Type: "finished",
			}.String()).Err()
			if err != nil {
				panic(err)
			}
		}

		// start initiating the process to receive a new file
		log.Debugf("working on file %d", c.FilesToTransferCurrentNum)

		// recipient requests the file and chunks (if empty, then should receive all chunks)
		bRequest, _ := json.Marshal(RemoteFileRequest{
			CurrentFileChunks:         c.CurrentFileChunks,
			FilesToTransferCurrentNum: c.FilesToTransferCurrentNum,
		})
		key, _ := c.Pake.SessionKey()
		err = c.redisdb.Publish(c.nameOutChannel, Message{
			Type:  "recipientready",
			Bytes: crypt.EncryptToBytes(bRequest, key),
		}.String()).Err()
		if err != nil {
			return
		}
		c.Step3RecipientRequestFile = true
		err = c.dataChannelReceive()
	}
	if c.Options.IsSender && c.Step3RecipientRequestFile && !c.Step4FileTransfer {
		log.Debug("start sending data!")
		err = c.dataChannelSend()
		c.Step4FileTransfer = true
	}
	return
}

func (c *Client) dataChannelReceive() (err error) {
	c.recvSess = receiver.NewWith(receiver.Config{})
	err = c.recvSess.CreateConnection()
	if err != nil {
		return
	}
	c.recvSess.CreateDataHandler()
	return
}

func (c *Client) dataChannelSend() (err error) {
	c.sendSess = sender.NewWith(sender.Config{
		Configuration: common.Configuration{
			OnCompletion: func() {
			},
		},
	})

	if err := c.sendSess.CreateConnection(); err != nil {
		log.Error(err)
		return err
	}
	if err := c.sendSess.CreateDataChannel(); err != nil {
		log.Error(err)
		return err
	}
	offer, err := c.sendSess.CreateOffer()
	if err != nil {
		log.Error(err)
		return err
	}

	// sending offer
	err = c.redisdb.Publish(c.nameOutChannel, Message{
		Type:    "datachannel-offer",
		Message: offer,
	}.String()).Err()
	if err != nil {
		return
	}

	return
}

// MissingChunks returns the positions of missing chunks.
// If file doesn't exist, it returns an empty chunk list (all chunks).
// If the file size is not the same as requested, it returns an empty chunk list (all chunks).
func MissingChunks(fname string, fsize int64, chunkSize int) (chunks []int64) {
	fstat, err := os.Stat(fname)
	if fstat.Size() != fsize {
		return
	}

	f, err := os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()

	buffer := make([]byte, chunkSize)
	emptyBuffer := make([]byte, chunkSize)
	chunkNum := 0
	chunks = make([]int64, int64(math.Ceil(float64(fsize)/float64(chunkSize))))
	var currentLocation int64
	for {
		bytesread, err := f.Read(buffer)
		if err != nil {
			break
		}
		if bytes.Equal(buffer[:bytesread], emptyBuffer[:bytesread]) {
			chunks[chunkNum] = currentLocation
		}
		currentLocation += int64(bytesread)
	}
	if chunkNum == 0 {
		chunks = []int64{}
	} else {
		chunks = chunks[:chunkNum]
	}
	return
}

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func Encode(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func Decode(in string, obj interface{}) (err error) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return
	}

	err = json.Unmarshal(b, obj)
	return
}
