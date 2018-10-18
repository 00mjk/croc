// +build wincroc

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/schollz/croc/src/cli"
	"github.com/schollz/croc/src/croc"
	"github.com/schollz/croc/src/utils"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

type CustomLabel struct {
	widgets.QLabel

	_ func(string) `signal:"updateTextFromGoroutine,auto(this.QLabel.setText)"` //TODO: support this.setText as well
}

var Version string

func main() {
	if len(os.Args) > 1 {
		cli.Run()
		return
	}

	var isWorking bool
	app := widgets.NewQApplication(len(os.Args), os.Args)

	window := widgets.NewQMainWindow(nil, 0)
	window.SetFixedSize2(400, 150)
	window.SetWindowTitle("🐊📦 croc " + Version)

	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(widgets.NewQVBoxLayout())
	window.SetCentralWidget(widget)

	labels := make([]*CustomLabel, 3)
	for i := range labels {
		label := NewCustomLabel(nil, 0)
		label.SetAlignment(core.Qt__AlignCenter)
		widget.Layout().AddWidget(label)
		labels[i] = label
	}
	labels[0].SetText("secure data transfer")
	labels[1].SetText("Click 'Send' or 'Receive' to start")

	button := widgets.NewQPushButton2("Send", nil)
	button.ConnectClicked(func(bool) {
		if isWorking {
			dialog("Can only do one send or receive at a time")
			return
		}
		isWorking = true

		var fileDialog = widgets.NewQFileDialog2(nil, "Open file to send...", "", "")
		fileDialog.SetAcceptMode(widgets.QFileDialog__AcceptOpen)
		fileDialog.SetFileMode(widgets.QFileDialog__AnyFile)
		if fileDialog.Exec() != int(widgets.QDialog__Accepted) {
			isWorking = false
			return
		}
		var fn = fileDialog.SelectedFiles()[0]
		if len(fn) == 0 {
			dialog(fmt.Sprintf("No file selected"))
			isWorking = false
			return
		}

		go func() {
			cr := croc.Init(false)
			done := make(chan bool)
			codePhrase := utils.GetRandomName()
			_, fname := filepath.Split(fn)
			labels[0].UpdateTextFromGoroutine(fmt.Sprintf("Sending '%s'", fname))
			labels[1].UpdateTextFromGoroutine(fmt.Sprintf("Code phrase: %s", codePhrase))

			go func(done chan bool) {
				for {
					if cr.FileInfo.SentName != "" {
						labels[0].UpdateTextFromGoroutine(fmt.Sprintf("Sending %s", cr.FileInfo.SentName))
					}
					if cr.Bar != nil {
						barState := cr.Bar.State()
						labels[1].UpdateTextFromGoroutine(fmt.Sprintf("%2.1f%% [%2.0f:%2.0f]", barState.CurrentPercent*100, barState.SecondsSince, barState.SecondsLeft))
					}
					labels[2].UpdateTextFromGoroutine(cr.StateString)
					time.Sleep(100 * time.Millisecond)
					select {
					case _ = <-done:
						labels[2].UpdateTextFromGoroutine(cr.StateString)
						return
					default:
						continue
					}
				}
			}(done)

			cr.Send(fn, codePhrase)
			done <- true
			isWorking = false
		}()
	})
	widget.Layout().AddWidget(button)

	receiveButton := widgets.NewQPushButton2("Receive", nil)
	receiveButton.ConnectClicked(func(bool) {
		if isWorking {
			dialog("Can only do one send or receive at a time")
			return
		}

		isWorking = true
		defer func() {
			isWorking = false
		}()

		// determine the folder to save the file
		var folderDialog = widgets.NewQFileDialog2(nil, "Open folder to receive file...", "", "")
		folderDialog.SetAcceptMode(widgets.QFileDialog__AcceptOpen)
		folderDialog.SetFileMode(widgets.QFileDialog__DirectoryOnly)
		if folderDialog.Exec() != int(widgets.QDialog__Accepted) {
			return
		}
		var fn = folderDialog.SelectedFiles()[0]
		if len(fn) == 0 {
			dialog(fmt.Sprintf("No folder selected"))
			return
		}

		var codePhrase = widgets.QInputDialog_GetText(nil, "croc", "Enter code phrase:",
			widgets.QLineEdit__Normal, "", true, core.Qt__Dialog, core.Qt__ImhNone)
		if len(codePhrase) < 3 {
			dialog(fmt.Sprintf("Invalid codephrase: '%s'", codePhrase))
			return
		}

		cwd, _ := os.Getwd()

		go func() {
			os.Chdir(fn)
			defer os.Chdir(cwd)

			cr := croc.Init(true)
			cr.WindowRecipientPrompt = true

			done := make(chan bool)

			go func(done chan bool) {
				for {
					if cr.WindowReceivingString != "" {
						var question = widgets.QMessageBox_Question(nil, "croc", fmt.Sprintf("%s?", cr.WindowReceivingString), widgets.QMessageBox__Yes|widgets.QMessageBox__No, 0)
						if question == widgets.QMessageBox__Yes {
							cr.WindowRecipientAccept = true
							labels[0].UpdateTextFromGoroutine(cr.WindowReceivingString)
						} else {
							cr.WindowRecipientAccept = false
							labels[2].UpdateTextFromGoroutine("canceled")
							return
						}
						cr.WindowRecipientPrompt = false
						cr.WindowReceivingString = ""
					}

					if cr.Bar != nil {
						barState := cr.Bar.State()
						labels[1].UpdateTextFromGoroutine(fmt.Sprintf("%2.1f%% [%2.0f:%2.0f]", barState.CurrentPercent*100, barState.SecondsSince, barState.SecondsLeft))
					}
					labels[2].UpdateTextFromGoroutine(cr.StateString)
					time.Sleep(100 * time.Millisecond)
					select {
					case _ = <-done:
						labels[2].UpdateTextFromGoroutine(cr.StateString)
						return
					default:
						continue
					}
				}
			}(done)

			cr.Receive(codePhrase)
			done <- true
			isWorking = false
		}()
	})
	widget.Layout().AddWidget(receiveButton)

	window.Show()
	app.Exec()
}

func dialog(s string) {
	var info = widgets.NewQMessageBox(nil)
	info.SetWindowTitle("Info")
	info.SetText(s)
	info.Exec()
}
