package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/soopsio/ssh"
)

func (config scpConfig) startSCPSource(s ssh.Session, opts scpOptions) error {
	var exitStatus uint8
	// We need to wait for client to initialize data transfer with a binary zero
	err := checkSCPClientCode(s)
	if err != nil {
		exitStatus = 1
		log.Printf("Got error receiving initial status code from client: %v", err)
		closeSession(s, exitStatus)
	}

	for _, target := range opts.fileNames {
		var absTarget string

		if !filepath.IsAbs(target) {
			absTarget = filepath.Clean(filepath.Join(config.Dir, target))
		} else {
			absTarget = target
		}

		absTarget = filepath.Clean(absTarget)
		if !strings.HasPrefix(absTarget, config.Dir) {
			// We've requested a file outside of our working directory, so deny it even exists!
			msg := fmt.Sprintf("scp: %s: No such file or directory", target)
			sendErrorToClient(msg, s)
			continue
		}

		log.Printf("Target is now %v - %v", absTarget, target)

		fileList, err := filepath.Glob(absTarget)
		if err != nil {
			log.Printf("Error when evaluating glob: %v", err)
			// Maybe a "file not found" isn't the most appropriate error to return here?
			msg := fmt.Sprintf("scp: %s: No such file or directory", target)
			sendErrorToClient(msg, s)
			continue
		}

		// If there are no matches it needs to be reported as an error (scp: <target>: No such file or directory)
		if len(fileList) == 0 {
			msg := fmt.Sprintf("scp: %s: No such file or directory", target)
			sendErrorToClient(msg, s)
		}

		for _, file := range fileList {
			// FIXME: We probably don't want to stop here, just log/report an error
			err := config.sendFileBySCP(file, s, opts)
			if err != nil {
				// TODO: Need to do something with the error here
			}
		}
	}

	closeSession(s, exitStatus)

	// TODO: We're not actually returning any errors here, maybe just change the func so it doesn't return anything
	if exitStatus != 0 {
		return errors.New("Errors were found")
	}
	return nil
}

// Close the session to the client returning a status code in the process
func closeSession(s ssh.Session, exitStatus uint8) {
	sendExitStatusCode(s, exitStatus)
	s.Close()
	log.Printf("session closed")
}

// Sends file modification and access times
func sendFileTimes(fi os.FileInfo, s ssh.Session) error {
	// TODO: This is not portable, need to figure out how this should behave in non-unix systems
	f, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		// TODO: Handle the error
		// Agghh!! We're not in unix!!
		log.Printf("We're not in unix")
		return errors.New("Not in a unix system, not sure what to do")
	}

	msg := fmt.Sprintf("T%d 0 %d 0\n", getLastModification(f).Sec, getLastAccess(f).Sec)
	err := sendSCPControlMsg(msg, s)
	return err
}

// Compose and send an scp control message
func composeSCPControlMsg(fi os.FileInfo, s ssh.Session, opts scpOptions) error {
	if opts.PreserveMode {
		err := sendFileTimes(fi, s)
		if err != nil {
			return err
		}
	}

	var msg string
	if fi.IsDir() {
		// TODO: We format mode as octal making sure it has a leading zero. What happens if sticky bit is already set?
		msg = fmt.Sprintf("D%#o 0 %v\n", fi.Mode()&os.ModePerm, fi.Name())
	} else {
		msg = fmt.Sprintf("C%#o %d %v\n", fi.Mode()&os.ModePerm, fi.Size(), fi.Name())
	}
	return sendSCPControlMsg(msg, s)
}

// Sends a scp control message and waits for the reply
func sendSCPControlMsg(msg string, s ssh.Session) error {
	log.Printf("Sending control message: %q", msg[:len(msg)-1])
	n, err := s.Write([]byte(msg))
	log.Printf("Sent %d bytes", n)
	if err != nil {
		return err
	}
	return checkSCPClientCode(s)
}

// Checks the status messages that the client is sending
// They can be as follows:
//   0: Everything's good
//   1: Warning (can be recovered from)
//   2: Fatal error (This will end the connection)
// 1 and 2 are followed by a text message (delimited by newline character)
// TODO: Differentiate between errors reported by the client or errors getting status from the client
func checkSCPClientCode(s ssh.Session) error {
	statusbuf := make([]byte, 1)
	nread, err := s.Read(statusbuf)
	if err != nil {
		return err
	}

	log.Printf("Received %d bytes from client", nread)

	// A binary 0 means everything is peachy
	if statusbuf[0] == 0 {
		return nil
	}

	// Got an error from the client: 1 (warning) or 2 (fatal)
	// Error is followed by an error message (delimited by a new line character)
	// TODO: Determine how big the buffer could/should be
	statusmsgbuf := make([]byte, 256)
	nread, err = s.Read(statusmsgbuf)
	msgSize := strings.Index(string(statusmsgbuf), "\n")
	msg := string(statusmsgbuf)[:msgSize]
	log.Printf("Got error %d from client: %v", statusbuf[0], msg)

	//TODO: Return a fatal error (special type) if we've received a 2 so we can close the connection
	return errors.New(msg)
}

// Notify the client of an error. Doesn't break the connection
func sendErrorToClient(msg string, s ssh.Session) error {
	_, err := s.Write([]byte("\001" + msg + "\n"))
	return err
}

// Send a file (or directory) through scp
func (config scpConfig) sendFileBySCP(file string, s ssh.Session, opts scpOptions) error {

	// Filename as the client sees it (used for error reporting purposes)
	filename := strings.TrimPrefix(file, config.Dir)

	f, err := os.Open(file)
	if err != nil {
		log.Printf("Open failed: %q", err)
		msg := fmt.Sprintf("scp: %s: %s", filename, err.(*os.PathError).Err)
		sendErrorToClient(msg, s)
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Printf("Stat failed: %q", err)
		msg := fmt.Sprintf("scp: %s: %s", filename, err.(*os.PathError).Err)
		sendErrorToClient(msg, s)
		return err
	}

	if fi.IsDir() {
		// We're trying to send a directory, this is either an error or we'll need to iterate through the directory's contents
		if !opts.Recursive {
			log.Printf("Found a dir but we're not being recursive (not a regular file): %q", file)

			msg := fmt.Sprintf("scp: %s: not a regular file", filename)
			sendErrorToClient(msg, s)
			return errors.New("not a regular file")
		}
		err := composeSCPControlMsg(fi, s, opts)

		if err != nil {
			// TODO: React accordingly (we probably don't want to keep sending this directory now)
			log.Printf("ERR is %q", err)
		}
		// TODO: Investigate if we might want to paginate this call in case there's a lot of files in there
		names, err := f.Readdirnames(0)
		log.Printf("Found the following files %v - (err is %v)", names, err)
		for _, name := range names {
			// TODO: Too many recursive calls might be a problem here.
			err := config.sendFileBySCP(filepath.Join(file, name), s, opts)
			if err != nil {
				// TODO: Handle this properly (check how scp does it)
				log.Printf("Got error after trying to send file: %q", err)
				return err
			}
		}
		// Signal that we've finished with this directory
		return sendSCPControlMsg("E\n", s)
	}
	// We're just sending a regular file
	err = composeSCPControlMsg(fi, s, opts)
	if err != nil {
		// TODO: React accordingly
		log.Printf("ERR is %q", err)
		return err
	}
	err = sendFileContentsBySCP(f, s)
	return err
}

// Does the actual data transfer of the file's contents
func sendFileContentsBySCP(f *os.File, s ssh.Session) error {
	n, err := io.Copy(s, f)
	log.Printf("Sending content, sent %d bytes", n)
	if err != nil {
		return err
	}

	// Need to send binary zero after actual data transfer to signify everything's ok
	_, err = s.Write([]byte("\000"))
	if err != nil {
		return err
	}

	return checkSCPClientCode(s)
}
