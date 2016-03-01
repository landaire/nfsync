package internal

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"

	"golang.org/x/crypto/ssh"
)

var (
	RemoteRoot string
	Host       string
	Config     *ssh.ClientConfig
	client     *ssh.Client
)

// UploadFiles takes a channel of file paths and uploads each file
// to the remote root, creating directories as needed
func UploadFiles(c <-chan string) {
	for {
		select {
		case file := <-c:
			if err := openClient(); err != nil {
				log.Errorf("Error occurred while opening connection: %v", err)
			}

			fmt.Println(file)

		case <-time.After(5 * time.Minute):
			closeClient()
		}
	}
}

func closeClient() {
	if err := client.Close(); err != nil {
		log.Errorf("Error occurred while closing client connection: %v", err)
	}

	client = nil
}

func openClient() error {
	if client != nil {
		return nil
	}

	var err error
	client, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", Host, 22), Config)

	return err
}
