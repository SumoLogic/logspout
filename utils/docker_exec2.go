package main

import (
	"bufio"
	"github.com/fsouza/go-dockerclient"
	"io"
	"io/ioutil"
	"log"
	"time"
)

func assert(err error, context string) {
	if err != nil {
		log.Print(context+": ", err)
	}
}

func main() {

	// Create the Docker client
	client, err := docker.NewClient("unix:///var/run/docker.sock")
	assert(err, "docker")

	// Create the execution object
	config := docker.CreateExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          []string{"/bin/sh", "-c", "cat - > /shizzle"},
		Container:    "sumo-logic-file-collector",
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
	go func() {
		err := client.StartExec(execObj.ID, opts)
		assert(err, "StartExec")
	}()

	// Create a stream pump
	NewStreamPump(outrd, errrd)

	file, err := ioutil.ReadFile("schnitzel")
	inwr.Write(file)
	inwr.Close()
	time.Sleep(100 * time.Second)
}

type StreamPump struct {
	Name string
}

func NewStreamPump(stdout, stderr io.Reader) *StreamPump {
	obj := &StreamPump{}
	pump := func(name string, source io.Reader) {
		log.Printf("pump: %s, starting\n", name)
		buf := bufio.NewReader(source)
		for {
			data, err := buf.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("pump: %s, %v\n", name, err)
				}
				return
			}
			log.Printf("pump: %s - %s", name, data)
		}
	}
	go pump("stdout", stdout)
	go pump("stderr", stderr)
	return obj
}
