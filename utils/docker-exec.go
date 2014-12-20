package main

import (
	"bufio"
	"github.com/fsouza/go-dockerclient"
	"io"
	"log"
	"sync"
)

func assert(err error, context string) {
	if err != nil {
		log.Fatal(context+": ", err)
	}
}

func main() {

	// Create the Docker client
	client, err := docker.NewClient("unix:///var/run/docker.sock")
	assert(err, "docker")

	// Create the execution object
	config := docker.CreateExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{"tail", "-F", "/opt/SumoCollector/logs/collector.log"},
		Container:    "sumo-logic-file-collector",
	}
	//Cmd:          []string{"ls", "-la", "/"},
	//Container:    "daemon_dave",

	execObj, err := client.CreateExec(config)
	assert(err, "CreateExec")

	// Setup the execution options
	outrd, outwr := io.Pipe()
	errrd, errwr := io.Pipe()
	opts := docker.StartExecOptions{
		Detach:       false,
		Tty:          false,
		InputStream:  nil,
		OutputStream: outwr,
		ErrorStream:  errwr,
		RawTerminal:  true,
		//Success:      success,
	}

	// Start the execution
	go func() {
		err := client.StartExec(execObj.ID, opts)
		assert(err, "StartExec")
	}()

	// Create a stream pump
	NewStreamPump(outrd, errrd)

	// Wait, wait, wait
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
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
