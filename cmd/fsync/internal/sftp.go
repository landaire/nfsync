package internal

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	// ConnectionTimeout is the max inactivity time allowed before the connection
	// to the SFTP server is closed
	ConnectionTimeout = 5 * time.Minute
	// FilesBuffer is the number of files that can be in the buffered chan
	FilesBuffer = 100
)

var (
	RemoteRoot    string
	Host          string
	Config        *ssh.ClientConfig
	ModifiedFiles chan string
	DeletedFiles  chan string
	client        *ssh.Client
)

func init() {
	ModifiedFiles = make(chan string, FilesBuffer)
	DeletedFiles = make(chan string, FilesBuffer)
}

// RemoteFileManager manages files on the server. This function handles the
// deleting and uploading of files/directories
func RemoteFileManager() {
	for {
		select {
		case file := <-ModifiedFiles:
			if err := openClient(); err != nil {
				Log.Errorf("Error occurred while opening connection: %v\n", err)
			}

			fmt.Println(file)

		case <-time.After(ConnectionTimeout):
			// After 5 minutes of inactivity, close the connection
			closeClient()
		}
	}
}

func closeClient() {
	if client == nil {
		return
	}

	if err := client.Close(); err != nil {
		Log.Errorf("Error occurred while closing client connection: %v\n", err)
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
