package server

import (
	"fmt"
	"log"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/soopsio/ssh"
)

type scpOptions struct {
	To           bool `short:"t"`
	From         bool `short:"f"`
	TargetIsDir  bool `short:"d"`
	Recursive    bool `short:"r"`
	PreserveMode bool `short:"p"`
	fileNames    []string
}

type scpConfig struct {
	Dir  string
	sess ssh.Session
}

func newScpConfig() *scpConfig {
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return &scpConfig{Dir: workingDir}
}

// Allows us to send to the client the exit status code of the command they asked as to run
func sendExitStatusCode(s ssh.Session, status uint8) {
	exitStatusBuffer := make([]byte, 4)
	exitStatusBuffer[3] = status
	_, err := s.SendRequest("exit-status", false, exitStatusBuffer)
	if err != nil {
		// TODO: Don't we prefer to return the error here?
		log.Printf("Failed to forward exit-status to client: %v", err)
	}
}

func initScpServer(dir string) scpConfig {
	config := scpConfig{
		Dir: dir,
	}
	return config
}

// Handle requests received through a s
func (config scpConfig) scpServerStart(s ssh.Session) {

	opts := scpOptions{}
	// TODO: Do a sanity check of options (like needing to have either -f or -t defined)
	// TODO: Define what happens if both -t and -f are specified?
	// TODO: If we have more than one filename with -t defined it's an error: "ambiguous target"

	// At the very least we expect either -t or -f
	// UNDOCUMENTED scp OPTIONS:
	//  -t: "TO", our server will be receiving files
	//  -f: "FROM", our server will be sending files
	//  -d: Target is expected to be a directory
	// DOCUMENTED scp OPTIONS:
	//  -r: Recursively copy entire directories (follows symlinks)
	//  -p: Preserve modification mtime, atime and mode of files
	// parseOpts := true
	// opts.fileNames = make([]string, 0)
	// for _, elem := range s.Command()[1:] {
	// 	if parseOpts {
	// 		switch elem {
	// 		case "-f":
	// 			opts.From = true
	// 		case "-t":
	// 			opts.To = true
	// 		case "-d":
	// 			opts.TargetIsDir = true
	// 		case "-p":
	// 			opts.PreserveMode = true
	// 		case "-r":
	// 			opts.Recursive = true
	// 		case "-v":
	// 		case "-q":
	// 			// Verbose mode, this is more of a local client thing
	// 		case "--":
	// 			// After finding a "--" we stop parsing for flags
	// 			if parseOpts {
	// 				parseOpts = false
	// 			} else {
	// 				opts.fileNames = append(opts.fileNames, elem)
	// 			}
	// 		default:
	// 			opts.fileNames = append(opts.fileNames, elem)
	// 		}
	// 	}
	// }
	args, err := flags.ParseArgs(&opts, s.Command()[1:])
	if err != nil {
		log.Println("Parse opts error: ", err)
	}
	fmt.Println(len(args), args, opts)
	opts.fileNames = args
	log.Printf("Called scp with %v", s.Command()[1:])
	log.Printf("Options: %v", opts)
	log.Printf("Filenames: %v", opts.fileNames)

	// We're acting as source
	if opts.From {
		err := config.startSCPSource(s, opts)
		_ = err
	}

	// We're acting as sink
	if opts.To {
		var statusCode uint8
		if len(opts.fileNames) != 1 {
			log.Printf("Error in number of targets (ambiguous target)")
			statusCode = 1
			sendErrorToClient("scp: ambiguous target", s)
		} else {
			config.startSCPSink(s, opts)
		}
		sendExitStatusCode(s, statusCode)
		s.Close()
		return
	}
}

// func main() {
// 	config := initSettings()
// 	serverConfig := config.initSSHConfig()
// 	startServer(config, serverConfig)
// }
