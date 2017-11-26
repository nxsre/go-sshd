package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/soopsio/ssh"
)

func sendSCPBinaryOK(s ssh.Session) error {
	_, err := s.Write([]byte("\000"))
	return err
}

type controlMessage struct {
	msgType string
	name    string
	mode    os.FileMode
	size    uint64
	mtime   int64
	atime   int64
}

// Generate a full path out of our basedir, the directories currently in the stack, and the target
func (config scpConfig) generatePath(dirStack []string, target string) string {

	var fullPathList []string
	fullPathList = append(fullPathList, config.Dir)
	fullPathList = append(fullPathList, dirStack...)
	fullPathList = append(fullPathList, target)
	path := filepath.Clean(filepath.Join(fullPathList...))
	return path
}

// Receive the contents of a file and store it in the right place
func (c scpConfig) receiveFileContents(s ssh.Session, dirStack []string, msgctrl controlMessage, name string, preserveMode bool) error {

	filename := c.generatePath(dirStack, name)

	log.Printf("Filename is '%s'", filename)

	// TODO: Make sure we're reporting the right error here if something happens
	f, err := os.Create(filename)
	if err != nil {
		log.Printf("Err is %v", err)
		return err
	}
	defer f.Close()
	nread, err := io.CopyN(f, s, int64(msgctrl.size))
	log.Printf("Transferred %d bytes", nread)
	if err != nil {
		log.Printf("Err is %v", err)
		return err
	}

	// TODO: Double check that we're doing the right thing in all cases (file already exists, file doesn't exist, etc)
	err = f.Chmod(msgctrl.mode)
	if err != nil {
		log.Printf("Err is %v", err)
		return err
	}

	if preserveMode {
		atime := time.Unix(msgctrl.atime, 0)
		mtime := time.Unix(msgctrl.mtime, 0)
		err := os.Chtimes(filename, atime, mtime)
		if err != nil {
			log.Printf("Err is %v", err)
			return err
		}
	}
	log.Println("1111111")

	statusbuf := make([]byte, 1)
	_, err = s.Read(statusbuf)
	if err != nil {
		log.Printf("Getting status error after transfer: %v", err)
		return err
	}
	sendSCPBinaryOK(s)
	log.Println("2222222")

	return err
}

// Create a directory, ignore errors if it already exists
func createDir(target string, ctrlmsg controlMessage) error {
	// TODO: What permissions should we use here?
	// var perm os.FileMode = 0755
	err := os.Mkdir(target, ctrlmsg.mode)
	if err != nil {
		// TODO: it's easier to compare to os.ErrExist
		if os.IsExist(err) {
			log.Printf("File already exists, big deal")
		} else {
			log.Printf("%v", err)
			return err
		}
	}
	return nil
}

// 判断文件夹是否存在
func directoryExist(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// If target exists and it's a dir, put all the files in there
// If target doesn't exist or it's a regular file:
//   - If we only want to copy one file, use it as destination
//   - If we want to copy more than one file, it's an error: "No such file or directory" or "Not a directory"
func (config scpConfig) startSCPSink(s ssh.Session, opts scpOptions) error {

	// Only one target should have been specified
	target := opts.fileNames[0]

	info, err := os.Stat(target)
	if err == nil {
		if info.IsDir() {
			target += string(os.PathSeparator)
		}
	}

	// Target seems to be a directory
	if string(target[len(target)-1]) == string(os.PathSeparator) {
		opts.TargetIsDir = true
	}

	absTarget := filepath.Clean(filepath.Join(config.Dir, target))
	if !strings.HasPrefix(absTarget, config.Dir) {
		// We're attempting to copy files outside of our working directory, so return an error
		msg := fmt.Sprintf("scp: %s: Not a directory", target)
		sendErrorToClient(msg, s)
		return errors.New(msg)
	}

	var dirStack []string
	if opts.TargetIsDir {
		// log.Println("aaaaaaaaa", dirStack)
		// err := createDir(absTarget, controlMessage{mode: 0755})
		// if err != nil {
		// 	return err
		// }
		dirStack = append(dirStack, target)
	}

	// Tell the other side we're ready to start receiving data
	sendSCPBinaryOK(s)
	for {
		// Steps here:
		// 1. Figure out what kind of message this is (file, directory, time, end of directory...)
		// If it's D:
		//   - Add a new directory to the stack
		//   - Create the directory
		// If it's E:
		//   - Remove one directory from the stack
		// If it's C:
		//   - Receive file and store it in the current stack
		// If it's T:
		//   - Receive the next control message

		ctrlmsg, err := receiveControlMsg(s)
		if err != nil {
			if err == io.EOF {
				// EOF is fine at this point, it just means no more files to copy
				break
			}
			log.Printf("Got error from client: %v", err)
			break
		}
		switch ctrlmsg.msgType {
		case "D":
			if !directoryExist(target) && opts.TargetIsDir == true {
				msg := fmt.Sprintf("scp: %s: No such file or directory", target)
				sendErrorToClient(msg, s)
				return errors.New(msg)
			}

			// 如果目录不存在，且 opts.TargetIsDir 是 false, 则认为需要重命名为 target, 同时需要修改 ctrlmsg, 设置 dirStack
			if !directoryExist(target) && opts.TargetIsDir == false {
				dir, file := filepath.Split(target)
				target = dir
				ctrlmsg.name = file
				dirStack = append(dirStack, target)
				opts.TargetIsDir = true
			}
			// TODO: Figure out how we need to behave in terms of permissions/times, etc
			err := createDir(config.generatePath(dirStack, ctrlmsg.name), ctrlmsg)
			if err != nil {
				sendErrorToClient(err.Error(), s)
				return err
			}
			dirStack = append(dirStack, ctrlmsg.name)

		case "E":
			stackSize := len(dirStack)
			if (opts.TargetIsDir && stackSize <= 1) || (!opts.TargetIsDir && stackSize <= 0) {
				msg := "scp: Protocol Error"
				sendErrorToClient(msg, s)
				return errors.New(msg)
			}
			dirStack = dirStack[:len(dirStack)-1]
		case "C":
			var filename string
			if opts.TargetIsDir {
				filename = ctrlmsg.name
			} else {
				filename = target
			}
			err := config.receiveFileContents(s, dirStack, ctrlmsg, filename, opts.PreserveMode)
			if err != nil {
				sendErrorToClient(err.Error(), s)
				return err
			}
		}

	}
	return nil
}

// 每次读取一行头控制信息
func receiveControlMsg(s ssh.Session) (controlMessage, error) {
	ctrlmsg := controlMessage{}
	bufios := bufio.NewReader(s)
	ctrlmsgbuf, _, err := bufios.ReadLine()
	if err != nil {
		// TODO: Maybe only return it upstream if it's an EOF, otherwise handle it?
		return ctrlmsg, err
	}
	ctrlmsg.msgType = string(ctrlmsgbuf[0])

	ctrlmsglist := strings.Split(string(ctrlmsgbuf), " ")
	log.Printf("控制信息列表: %v", ctrlmsglist)

	if len(ctrlmsglist) > 3 {
		ctrlmsglist = ctrlmsglist[:3]
	}
	log.Printf("前三段有效控制信息: %v", ctrlmsglist)

	// Make sure control message is valid
	switch string(ctrlmsg.msgType) {
	case "E":
		sendSCPBinaryOK(s)
		if len(ctrlmsgbuf) > 2 {
			log.Printf("Protocol error, got: %v", string(ctrlmsgbuf))
			return ctrlmsg, errors.New("Protocol error")
		}
		return ctrlmsg, nil
	case "C":
	case "D":
	case "T":
		ctrlmsg.mtime, err = strconv.ParseInt(ctrlmsglist[0][1:], 10, 64)
		if err != nil {
			return ctrlmsg, errors.New("mtime.sec not delimited")
		}
		ctrlmsg.atime, err = strconv.ParseInt(ctrlmsglist[2], 10, 64)
		if err != nil {
			return ctrlmsg, errors.New("atime.sec not delimited")
		}
		sendSCPBinaryOK(s)
		// A "T" message will always come before a "D" or "C", so we can combine both
		newCtrlmsg := controlMessage{}
		newCtrlmsg, err = receiveControlMsg(s)
		if err != nil {
			log.Println("newCtrlmsg error:", err)
			return ctrlmsg, errors.New("newCtrlmsg error,Protocol error")
		}
		newCtrlmsg.mtime = ctrlmsg.mtime
		newCtrlmsg.atime = ctrlmsg.atime

		return newCtrlmsg, nil
	default:
		// TODO: We have an expected message here, report it and abort
	}

	if len(ctrlmsglist) != 3 {
		log.Println("len(ctrlmsglist) != 3")
		return ctrlmsg, errors.New("ctrlmsglist illegal,Protocol error")
	}

	ctrlmsg.name = ctrlmsglist[2] // Remove trailing newline
	size, err := strconv.ParseInt(ctrlmsglist[1], 10, 64)
	ctrlmsg.size = uint64(size)
	if err != nil {
		return ctrlmsg, errors.New("Protocol error")
	}

	mode, err := strconv.ParseInt(ctrlmsglist[0][1:], 8, 32)
	ctrlmsg.mode = os.FileMode(mode)
	if err != nil {
		return ctrlmsg, errors.New("Protocol error")
	}
	sendSCPBinaryOK(s)
	return ctrlmsg, nil
}
