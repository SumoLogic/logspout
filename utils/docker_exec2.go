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

	attachStdin := buf != nil
	attachOut := outChannel != nil
	attachErr := errChannel != nil

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

	success := make(chan struct{})
	go func() {
		err := client.StartExec(execObj.ID, opts)
		assert(err, "StartExec")
		close(success)
	}()

	newStreamPump(outrd, errrd, outChannel, errChannel)

	if attachStdin {
		inwr.Write(buf)
		inwr.Close()
	}

	<-success
}

type streamPump struct {
	Name string
}

func newStreamPump(stdout, stderr io.Reader,
	outChannel *chan string, errChannel *chan string) *streamPump {

	obj := &streamPump{}
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
			*channel <- data
		}
	}
	go pump("stdout", stdout, outChannel)
	go pump("stderr", stderr, errChannel)
	return obj
}

func consume(tag string, channel *chan string) {
	for line := range *channel {
		log.Printf("%s: %s", tag, line)
	}
}

func main() {

	if len(os.Args) < 2 {
		log.Panicf("No container name specified\n")
	}

	client, err := docker.NewClient("unix:///var/run/docker.sock")
	assert(err, "docker")

	container := os.Args[1]
	dropcmd := []string{"/bin/sh", "-c", "cat - > /shizzle"}
	file, err := ioutil.ReadFile("schnitzel")
	assert(err, "ReadFile")
	dockerExec(client, container, dropcmd, file, nil, nil)

	tailcmd := []string{"/bin/sh", "-c", "chmod o+x /shizzle && /shizzle"}
	outChannel := make(chan string)
	go consume("stdout", &outChannel)
	errChannel := make(chan string)
	go consume("stderr", &errChannel)
	dockerExec(client, container, tailcmd, nil, &outChannel, &errChannel)

}
