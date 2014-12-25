package main

import (
	"bufio"
	"github.com/fsouza/go-dockerclient"
	"io"
	"io/ioutil"
	"log"
	"os"
)

func assert(err error, context string) {
	if err != nil {
		log.Printf("Failed assertion - %s: %s", context, err)
	}
}

func dockerExec(client *docker.Client, container string, cmd []string,
	buf []byte, outChannel *chan string, errChannel *chan string) {

	// Figure out whether we need stdin
	attachStdin := buf != nil
	attachOut := outChannel != nil
	attachErr := errChannel != nil

	// Create the execution object
	config := docker.CreateExecOptions{
		AttachStdin:  attachStdin,
		AttachStdout: attachOut,
		AttachStderr: attachErr,
		Tty:          false,
		Cmd:          cmd,
		Container:    container,
	}
	execObj, err := client.CreateExec(config)
	assert(err, "CreateExec")

	// Setup the execution options
	inrd, inwr := io.Pipe()
	outrd, outwr := io.Pipe()
	errrd, errwr := io.Pipe()
	opts := docker.StartExecOptions{
		Detach:       false,
		Tty:          false,
		InputStream:  inrd,
		OutputStream: outwr,
		ErrorStream:  errwr,
		RawTerminal:  false,
	}

	// Start the execution
	success := make(chan struct{})
	go func() {
		err := client.StartExec(execObj.ID, opts)
		assert(err, "StartExec")
		close(success)
	}()

	// Make sure we capture all output
	NewStreamPump(outrd, errrd, outChannel, errChannel)

	// Write into stdin
	if attachStdin {
		inwr.Write(buf)
		inwr.Close()
	}

	// Wait for the execution to finish
	<-success
}

type StreamPump struct {
	Name string
}

func NewStreamPump(stdout, stderr io.Reader,
	outChannel *chan string, errChannel *chan string) *StreamPump {

	obj := &StreamPump{}
	pump := func(name string, source io.Reader, channel *chan string) {
		buf := bufio.NewReader(source)
		for {
			data, err := buf.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("pump: %s, %v\n", name, err)
				}
				return
			}
			//log.Printf("pump: %s - %s", name, data)
			*channel <- data
		}
	}
	go pump("stdout", stdout, outChannel)
	go pump("stderr", stderr, errChannel)
	return obj
}

func main() {

	// Create the Docker client
	client, err := docker.NewClient("unix:///var/run/docker.sock")
	assert(err, "docker")

	// Setup the file drop
	container := os.Args[1]
	dropcmd := []string{"/bin/sh", "-c", "cat - > /shizzle"}
	file, err := ioutil.ReadFile("schnitzel")
	assert(err, "ReadFile")
	dockerExec(client, container, dropcmd, file, nil, nil)

	// Execute the file
	tailcmd := []string{"/bin/sh", "-c", "chmod o+x /shizzle && /shizzle"}
	outChannel := make(chan string)
	go func() {
		for line := range outChannel {
			log.Printf("stdout: %s", line)
		}
	}()
	errChannel := make(chan string)
	go func() {
		for line := range errChannel {
			log.Printf("stderr: %s", line)
		}
	}()
	dockerExec(client, container, tailcmd, nil, &outChannel, &errChannel)

}
