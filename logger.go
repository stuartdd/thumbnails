package main

import (
	"fmt"
	_ "image/png"
	"os"
	"strings"
	"sync"
	"time"
)

type LFFlag int

const (
	USE_F1 LFFlag = iota
	USE_F2
	USE_NONE
)

type LFFileData struct {
	file        *os.File
	fileName    string
	notifyError func(string, string, error)
}

type LFWriter struct {
	use            LFFlag
	file1          *LFFileData
	file2          *LFFileData
	fileMask       string
	lock           sync.Mutex
	monitorRunning bool
	monitorDelay   time.Duration
}

func (fd *LFFileData) notify(id, file string, err error) error {
	if fd.notifyError != nil {
		go func() {
			time.Sleep(100 * time.Millisecond)
			fd.notifyError(id, file, err)
		}()
	}
	return err
}

func (fd *LFFileData) requiresChange(newName string) bool {
	if fd.fileName == "" {
		return false
	}
	return newName != fd.fileName
}

func (fd *LFFileData) write(b []byte) (int, error) {
	if fd.file != nil {
		return fd.file.Write(b)
	}
	return os.Stderr.Write(b)
}

func (fd *LFFileData) open(name string) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fd.notify("OPEN", name, err)
	} else {
		fd.file = f
		fd.fileName = name
	}
	return nil
}

func (fd *LFFileData) close() {
	if fd.file == nil {
		fd.notify("CLOSE", fd.fileName, fmt.Errorf("file is nil"))
		return
	}
	err := fd.file.Close()
	if err != nil {
		fd.notify("CLOSE", fd.fileName, err)
	}
	fd.file = nil
	if fd.fileName == "" {
		fd.notify("STAT", fd.fileName, fmt.Errorf("filename is \"\""))
		return
	}
	s, err := os.Stat(fd.fileName)
	if err != nil {
		fd.notify("STAT", fd.fileName, err)
	} else {
		if s.Size() < 10 {
			err := os.Remove(fd.fileName)
			if err != nil {
				fd.notify("REMOVE", fd.fileName, err)
			}
		}
	}
}

//
// Create the LogFile Writer.
//
func NewLFWriter(fileMask string, delaySec uint, notifyError func(string, string, error)) (*LFWriter, error) {
	if delaySec < 5 {
		return nil, fmt.Errorf("log rotation check must be above 5 seconds")
	}
	f1 := &LFFileData{file: nil, fileName: "", notifyError: notifyError}
	f2 := &LFFileData{file: nil, fileName: "", notifyError: notifyError}
	lfw := &LFWriter{fileMask: fileMask, file1: f1, file2: f2, monitorDelay: time.Duration(delaySec * uint(time.Second)), monitorRunning: false}
	err := lfw.file1.open(_deriveNameForLogWriter(lfw.fileMask))
	if err != nil {
		return nil, err
	}
	lfw.use = USE_F1
	lfw.startMonitor()
	return lfw, nil
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
	lfw.use = USE_NONE
	lfw.monitorRunning = false // Stop the monitor running
	lfw.file1.close()
	lfw.file2.close()
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
	switch lfw.use {
	case USE_F1:
		return lfw.file1.write(b)
	case USE_F2:
		return lfw.file2.write(b)
	}
	return os.Stderr.Write(b)
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
			newName := _deriveNameForLogWriter(lfw.fileMask)
			switch lfw.use {
			case USE_F1:
				if lfw.file1.requiresChange(newName) {
					// Start using file 2
					err := lfw.file2.open(newName)
					if err == nil {
						lfw.use = USE_F2
						lfw.file1.close()
					}
				}
			case USE_F2:
				if lfw.file2.requiresChange(newName) {
					// Start using file1
					err := lfw.file1.open(newName)
					if err == nil {
						lfw.use = USE_F1
						lfw.file2.close()
					}
				}
			}
			time.Sleep(lfw.monitorDelay)
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
