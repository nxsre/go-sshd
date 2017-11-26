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

// TODO: Cleanup, send errors on protocol errors
func receiveControlMsg(s ssh.Session) (controlMessage, error) {
	ctrlmsg := controlMessage{}

	/* 	rs := bufio.NewReader(s)
	   	ctrlmsgbuf, isprefix, err := rs.ReadLine()
	   	log.Println("ctrlmsgbuf::", ctrlmsgbuf, string(ctrlmsgbuf), isprefix, err)
	   	if err == io.EOF {
	   		return ctrlmsg, nil
	   	}
	   	ctrlmsg.msgType = string(ctrlmsgbuf[0])
	   	ctrlmsglist := strings.Split(string(ctrlmsgbuf), " ") */
	// ctrlmsgbuf := make([]byte, 256)
	// nread, err := s.Read(ctrlmsgbuf)
	bufios := bufio.NewReader(s)
	ctrlmsgbuf, err := bufios.ReadSlice('\n')
	if err != nil {
		// TODO: Maybe only return it upstream if it's an EOF, otherwise handle it?
		return ctrlmsg, err
	}
	nread := len(ctrlmsgbuf)
	ctrlmsg.msgType = string(ctrlmsgbuf[0])

	ctrlmsglist := strings.Split(string(ctrlmsgbuf[:nread]), " ")
	nextctrlmsglist := []string{}

	log.Printf("ctrlmsglist_aa: %v", ctrlmsglist)

	if len(ctrlmsglist) > 3 {
		nextctrlmsglist = ctrlmsglist[3:]

		ctrlmsglist = ctrlmsglist[:3]
	}
	log.Printf("ctrlmsglist: %v", ctrlmsglist)

	// Make sure control message is valid
	switch string(ctrlmsgbuf[0]) {
	case "E":
		fmt.Println(string(ctrlmsgbuf), ctrlmsgbuf)
		if nread > 2 {
			// TODO: Protocol error
			ctrls := strings.Split(string(ctrlmsgbuf[:nread]), "\n")
			for _, ctrl := range ctrls {

				log.Println([]byte(ctrl), nread)

				if len(ctrl) > 1 {
					log.Printf("Protocol error, got: %v", string(ctrlmsgbuf[:nread]))
					return ctrlmsg, errors.New("Protocol error")
				}
				err := sendSCPBinaryOK(s)
				log.Println(err)
			}
		}
		/* 	if len(ctrlmsgbuf) > 2 {
			log.Printf("Protocol error, got: %v", string(ctrlmsgbuf))
			return ctrlmsg, errors.New("Protocol error")
		} */
		return ctrlmsg, nil
	case "C":
		if len(ctrlmsglist) != 3 {
			log.Println("len(ctrlmsglist) != 3")
			return ctrlmsg, errors.New("ctrlmsglist illegal,Protocol error")
		}

		ctrlmsg.name = ctrlmsglist[2][:len(ctrlmsglist[2])-1] // Remove trailing newline

		// ctrlmsg.name = ctrlmsglist[2]
		size, err := strconv.ParseInt(ctrlmsglist[1], 10, 64)
		ctrlmsg.size = uint64(size)
		if err != nil {
			return ctrlmsg, errors.New("Protocol error")
		}

		fmt.Println("ctrlmsglist[0][1:]", ctrlmsglist[0][1:])
		mode, err := strconv.ParseInt(ctrlmsglist[0][1:], 8, 32)
		ctrlmsg.mode = os.FileMode(mode)
		if err != nil {
			return ctrlmsg, errors.New("Protocol error")
		}
		sendSCPBinaryOK(s)
	case "D":
		if len(ctrlmsglist) != 3 {
			log.Println("len(ctrlmsglist) != 3")
			return ctrlmsg, errors.New("ctrlmsglist illegal,Protocol error")
		}

		ctrlmsg.name = ctrlmsglist[2][:len(ctrlmsglist[2])-1] // Remove trailing newline
		// ctrlmsg.name = ctrlmsglist[2]
		size, err := strconv.ParseInt(ctrlmsglist[1], 10, 64)
		ctrlmsg.size = uint64(size)
		if err != nil {
			return ctrlmsg, errors.New("Protocol error")
		}

		fmt.Println("ctrlmsglist[0][1:]", ctrlmsglist[0][1:])
		mode, err := strconv.ParseInt(ctrlmsglist[0][1:], 8, 32)
		ctrlmsg.mode = os.FileMode(mode)
		if err != nil {
			return ctrlmsg, errors.New("Protocol error")
		}
		sendSCPBinaryOK(s)

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
		log.Println("444444444444", len(nextctrlmsglist), nextctrlmsglist)
		if len(nextctrlmsglist) >= 3 {
			ctrlmsglist = nextctrlmsglist[:3]
			fmt.Println("#############", ctrlmsglist)
			mode, err := strconv.ParseInt(ctrlmsglist[0][3:], 8, 32)
			if err != nil {
				return newCtrlmsg, errors.New("1111 Protocol error")
			}

			log.Println("ctrlmsglist[1]", ctrlmsglist[1])
			size, err := strconv.ParseUint(ctrlmsglist[1], 10, 64)
			if err != nil {
				log.Println(err)
				return newCtrlmsg, errors.New("22222 Protocol error")
			}
			newCtrlmsg = controlMessage{
				msgType: string(ctrlmsglist[0][2]),
				mode:    os.FileMode(mode),
				size:    size,
				name:    strings.TrimRight(ctrlmsglist[2], "\n"),
			}
		} else {
			newCtrlmsg, err = receiveControlMsg(s)
			if err != nil {
				log.Println("newCtrlmsg error:", err)
				return ctrlmsg, errors.New("newCtrlmsg error,Protocol error")
			}

		}
		sendSCPBinaryOK(s)

		newCtrlmsg.mtime = ctrlmsg.mtime
		newCtrlmsg.atime = ctrlmsg.atime
		log.Println("AAAAAA:", newCtrlmsg, ctrlmsg)

		return newCtrlmsg, nil

	default:
		// TODO: We have an expected message here, report it and abort
	}

	// if ctrlmsg.msgType == "T" {

	// }

	/* 	if len(ctrlmsglist) != 3 {
	   		log.Println("len(ctrlmsglist) != 3")
	   		return ctrlmsg, errors.New("ctrlmsglist illegal,Protocol error")
	   	}

	   	ctrlmsg.name = ctrlmsglist[2][:len(ctrlmsglist[2])-1] // Remove trailing newline
	   	// ctrlmsg.name = ctrlmsglist[2]
	   	size, err := strconv.ParseInt(ctrlmsglist[1], 10, 64)
	   	ctrlmsg.size = uint64(size)
	   	if err != nil {
	   		debug.PrintStack()
	   		return ctrlmsg, errors.New("Protocol error")
	   	}

	   	fmt.Println("ctrlmsglist[0][1:]", ctrlmsglist[0][1:])
	   	mode, err := strconv.ParseInt(ctrlmsglist[0][1:], 8, 32)
	   	ctrlmsg.mode = os.FileMode(mode)
	   	if err != nil {
	   		debug.PrintStack()

	   		return ctrlmsg, errors.New("Protocol error")
	   	}
	   	sendSCPBinaryOK(s) */
	return ctrlmsg, nil
}

// Generate a full path out of our basedir, the directories currently in the stack, and the target
func (config scpConfig) generatePath(dirStack []string, target string) string {
	fmt.Println("DIRSTACK:", dirStack)
	fmt.Println("target:", target)
	var fullPathList []string
	fullPathList = append(fullPathList, config.Dir)
	fullPathList = append(fullPathList, dirStack...)
	fullPathList = append(fullPathList, target)
	fmt.Println("fullPathList:", fullPathList)

	path := filepath.Clean(filepath.Join(fullPathList...))
	fmt.Println("path:", path)

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
	fmt.Println("msgctrl.size", msgctrl)
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
	sendSCPBinaryOK(s)

	statusbuf := make([]byte, 1)
	_, err = s.Read(statusbuf)
	if err != nil {
		log.Printf("Getting status error after transfer: %v", err)
		return err
	}
	sendSCPBinaryOK(s)
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
		err := createDir(absTarget, controlMessage{mode: 0755})
		if err != nil {
			return err
		}
		dirStack = append(dirStack, target)
	}

	// Tell the other side we're ready to start receiving data
	sendSCPBinaryOK(s)
	i := 1
	for {
		fmt.Println(i)
		i++
		ctrlmsg, err := receiveControlMsg(s)

		if err != nil {
			if err == io.EOF {
				// EOF is fine at this point, it just means no more files to copy
				break
			}
			log.Printf("Got error from client: %v", err)
			break
		}

		log.Printf("Message type: %v", ctrlmsg.msgType)
		log.Println("Message: ", ctrlmsg, dirStack)
		switch ctrlmsg.msgType {
		case "D":
			log.Println("创建目录", dirStack, ctrlmsg.name, opts.fileNames)
			opts.TargetIsDir = true
			// 判断目标地址是否存在，如果不存在则创建，并且设置 dirStack
			if !directoryExist(target) {
				err := createDir(target, ctrlmsg)
				if err != nil {
					return err
				}
				dir, file := filepath.Split(target)
				target = file
				dirStack = append(dirStack, dir, file)
			} else {
				// TODO: Figure out how we need to behave in terms of permissions/times, etc
				err := createDir(config.generatePath(dirStack, ctrlmsg.name), ctrlmsg)
				if err != nil {
					return err
				}
				dirStack = append(dirStack, ctrlmsg.name)
			}
			log.Printf("dir stack is now: %v", dirStack)
		case "E":
			stackSize := len(dirStack)
			if (opts.TargetIsDir && stackSize <= 1) || (!opts.TargetIsDir && stackSize <= 0) {
				msg := "scp: Protocol Error"
				sendErrorToClient(msg, s)
				return errors.New(msg)
			}
			dirStack = dirStack[:len(dirStack)-1]
			fmt.Println("66666666666666666666")
		case "C":
			var filename string
			if opts.TargetIsDir {
				filename = ctrlmsg.name
			} else {
				filename = target
			}
			log.Println("创建文件", "dirStack", dirStack, "ctrlmsg.name", ctrlmsg.name, "opts.fileNames", opts.fileNames, "filename", filename)
			err := config.receiveFileContents(s, dirStack, ctrlmsg, filename, opts.PreserveMode)
			fmt.Println("AAAAAAAAAAAa", err)
			if err != nil {
				sendErrorToClient(err.Error(), s)
				return err
			}
		}

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

	}
	return nil
}
