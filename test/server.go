package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type connectionMap struct {
	reciever map[string]net.Conn
	sender   map[string]net.Conn
	sync.RWMutex
}

var connections connectionMap

func init() {
	connections.Lock()
	connections.reciever = make(map[string]net.Conn)
	connections.sender = make(map[string]net.Conn)
	connections.Unlock()
}

func runServer() {
	logger := log.WithFields(log.Fields{
		"function": "main",
	})
	logger.Info("Initializing")
	var wg sync.WaitGroup
	wg.Add(numberConnections)
	for id := 0; id < numberConnections; id++ {
		go listenerThread(id, &wg)
	}
	wg.Wait()
}

func listenerThread(id int, wg *sync.WaitGroup) {
	logger := log.WithFields(log.Fields{
		"function": "listenerThread@" + serverAddress + ":" + strconv.Itoa(27000+id),
	})

	defer wg.Done()
	err := listener(id)
	if err != nil {
		logger.Error(err)
	}
}

func listener(id int) (err error) {
	port := strconv.Itoa(27001 + id)
	logger := log.WithFields(log.Fields{
		"function": "listener@" + serverAddress + ":" + port,
	})
	server, err := net.Listen("tcp", serverAddress+":"+port)
	if err != nil {
		return errors.Wrap(err, "Error listening on "+serverAddress+":"+port)
	}
	defer server.Close()
	logger.Info("waiting for connections")
	//Spawn a new goroutine whenever a client connects
	for {
		connection, err := server.Accept()
		if err != nil {
			return errors.Wrap(err, "problem accepting connection")
		}
		logger.Infof("Client %s connected", connection.RemoteAddr().String())
		go clientCommuncation(id, connection)
	}
}

func clientCommuncation(id int, connection net.Conn) {
	sendMessage("who?", connection)
	message := receiveMessage(connection)
	connectionType := strings.Split(message, ".")[0]
	codePhrase := strings.Split(message, ".")[1]
	// If reciever
	connections.Lock()
	connections.reciever[codePhrase] = connection
	connections.Unlock()

	if connectionType == "s" {
		// periodically check if the receiver has joined

	}
	fmt.Println(message)
	return
}

func sendMessage(message string, connection net.Conn) {
	message = fillString(message, 64)
	connection.Write([]byte(message))
}

func receiveMessage(connection net.Conn) string {
	messageByte := make([]byte, 64)
	connection.Read(messageByte)
	return strings.Replace(string(messageByte), ":", "", -1)
}

func fillString(retunString string, toLength int) string {
	for {
		lengthString := len(retunString)
		if lengthString < toLength {
			retunString = retunString + ":"
			continue
		}
		break
	}
	return retunString
}
