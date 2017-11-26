package server

import (
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
	log.Println(opts)
	args, err := flags.ParseArgs(&opts, s.Command()[1:])
	if err != nil {
		log.Println("Parse opts error: ", err)
	}
	log.Println(len(args), args, opts)
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
			err := config.startSCPSink(s, opts)
			if err != nil {
				statusCode = 1
				log.Println(err)
			}
		}
		// 终止传输
		sendExitStatusCode(s, statusCode)
		s.Write([]byte("\002\n"))
	}
}

// func main() {
// 	config := initSettings()
// 	serverConfig := config.initSSHConfig()
// 	startServer(config, serverConfig)
// }
// 	config := initSettings()
// 	serverConfig := config.initSSHConfig()
// 	startServer(config, serverConfig)
// }
//
