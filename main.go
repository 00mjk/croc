package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

const BUFFERSIZE = 1024
const numberConnections = 4

// Build flags
var server, file string

// Global varaibles
var serverAddress, fileName, codePhraseFlag, connectionTypeFlag string
var runAsRelay, debugFlag bool

func main() {
	flag.BoolVar(&runAsRelay, "relay", false, "run as relay")
	flag.BoolVar(&debugFlag, "debug", false, "debug mode")
	flag.StringVar(&serverAddress, "server", "cowyo.com", "address of relay server")
	flag.StringVar(&fileName, "send", "", "file to send")
	flag.StringVar(&codePhraseFlag, "code", "", "use your own code phrase")
	flag.Parse()
	// Check build flags too, which take precedent
	if server != "" {
		serverAddress = server
	}
	if file != "" {
		fileName = file
	}

	if len(fileName) > 0 {
		connectionTypeFlag = "s" // sender
	} else {
		connectionTypeFlag = "r" //receiver
	}

	if !runAsRelay {
		if len(codePhraseFlag) == 0 {
			if connectionTypeFlag == "r" {
				codePhraseFlag = getInput("What is your code phrase? ")
			}
			if len(codePhraseFlag) < 5 {
				codePhraseFlag = GetRandomName()
				fmt.Println("Your code phrase is now " + codePhraseFlag)
			}
		}
	}

	if connectionTypeFlag == "s" {
		// encrypt the file
		fdata, err := ioutil.ReadFile(fileName)
		if err != nil {
			log.Fatal(err)
			return
		}
		encrypted, err := Encrypt(fdata, codePhraseFlag)
		if err != nil {
			log.Fatal(err)
			return
		}
		ioutil.WriteFile(fileName+".encrypted", encrypted, 0644)
	}

	log.SetFormatter(&log.TextFormatter{})
	if debugFlag {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}

	if runAsRelay {
		runServer()
	} else if len(serverAddress) != 0 {
		runClient(connectionTypeFlag, codePhraseFlag)
	} else {
		fmt.Println("You must specify either -file (for running as a server) or -server (for running as a client)")
	}
}

func getInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}
