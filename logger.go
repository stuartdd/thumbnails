package main

import (
	"fmt"
	_ "image/png"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type LFWriter struct {
	file           *os.File
	fallback       io.Writer
	currentName    string
	fileMask       string
	lock           sync.Mutex
	monitorRunning bool
	delay          time.Duration
}

//
// Create the LogFile Writer.
//
func NewLFWriter(fileMask string, delaySec uint) (*LFWriter, error) {
	if delaySec < 5 {
		return nil, fmt.Errorf("log rotation check must be above 5 seconds")
	}
	return &LFWriter{fileMask: fileMask, currentName: _deriveNameForLogWriter(fileMask), file: nil, delay: time.Duration(delaySec * uint(time.Second)), monitorRunning: false, fallback: log.Writer()}, nil
}

//
// Close the log file.
// Stop the monitor thread
// Restore the original log writer. This means that this Write will never be called again.
// Close the current log file and report any error via the log (and the original log writer)
//
func (lfw *LFWriter) CloseLogWriter() {
	lfw.lock.Lock()
	defer lfw.lock.Unlock()
	lfw.monitorRunning = false  // Stop the monitor running
	log.SetOutput(lfw.fallback) // Restore the log writer so all future Writes to log do not go via this LFWriter
	if lfw.file == nil {
		return
	}
	if lfw.file != nil { // If the file exists then close it
		err := lfw.file.Close() // Close and report errors to fallback writer via log
		lfw.file = nil          // Ensure file is null
		if err != nil {
			log.Printf("Failed to close log file %s. Fallback is active. Error:%e", lfw.currentName, err)
		}
	}
}

//
// Dont create the log file untill we absolutly need to.
// This stops files being created if the file name changes but no writes are requested.
//
// Write to the log file. The log API will call this if you:
//    lfw, err := NewLFWriter(fileMask, monitorDurationSeconds)
//    checkError(err)
//    log.SetOutput(lfw)
//
//  If the file is not opened (file==nil) then create it.
// 	If an error occurs, restore the log writer. Log the error
//
func (lfw *LFWriter) Write(b []byte) (n int, err error) {
	lfw.lock.Lock()
	defer lfw.lock.Unlock()

	if lfw.file == nil {
		fileName := _deriveNameForLogWriter(lfw.fileMask)
		logFile, err := os.OpenFile(fileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if strings.Contains(string(b), "application/json") {
			err = fmt.Errorf("rouge error")
		}
		if err != nil {
			log.SetOutput(lfw.fallback)                                                // Restore log writer
			written, _ := lfw.fallback.Write(b)                                        // log the line
			log.Printf("Failed to create log file %s. Error:%e", lfw.currentName, err) // log the error
			return written, err
		}
		lfw.file = logFile // Set the file and start the monitor thread
		lfw.currentName = fileName
		if !lfw.monitorRunning {
			lfw.startMonitor()
		}
	}
	return lfw.file.Write(b) // Write to the file
}

//
// Derive a new file name. If different from the current file name then:
//   Close the current file. It will be opened on the next write
//   Stop the monitor thread. It will be started again on the next Write if no error occurs.
//   Set the currentname to the new name.
//   Wait for the next Write!
//
func (lfw *LFWriter) startMonitor() {
	lfw.monitorRunning = true
	go func() {
		for lfw.monitorRunning {
			fmt.Println("Ping:")
			time.Sleep(lfw.delay)
			newName := _deriveNameForLogWriter(lfw.fileMask)
			if newName != lfw.currentName {
				if lfw.file != nil {
					lfw.file.Close() // Cannot do anything here if an error occurs!
					lfw.file = nil
				}
				lfw.monitorRunning = false
				lfw.currentName = newName
				fmt.Printf("File name change : %s\n", lfw.currentName)
			}
		}
	}()
}

func _deriveNameForLogWriter(mask string) string {
	t := time.Now()
	lfn := strings.ReplaceAll(mask, "%y", fmt.Sprintf("%d", t.Year()))
	lfn = strings.ReplaceAll(lfn, "%d", fmt.Sprintf("%d", t.YearDay()))
	lfn = strings.ReplaceAll(lfn, "%h", fmt.Sprintf("%d", t.Hour()))
	lfn = strings.ReplaceAll(lfn, "%m", fmt.Sprintf("%d", t.Minute()))
	return lfn
}
