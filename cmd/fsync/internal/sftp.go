package internal

import (
	"fmt"
	"path"
	"time"

	"github.com/pkg/sftp"

	"golang.org/x/crypto/ssh"
)

const (
	// ConnectionTimeout is the max inactivity time allowed before the connection
	// to the SFTP server is closed
	ConnectionTimeout = 5 * time.Minute
	// FilesBuffer is the number of files that can be in the buffered chan
	FilesBuffer = 100
	// ConcurrentOperations is the number of remote operations that can
	// happen concurrently
	ConcurrentOperations = 5
)

var (
	RemoteRoot            string
	Host                  string
	Config                *ssh.ClientConfig
	ModifiedFiles         chan string
	DeletedFiles          chan string
	RemoteFileManagerExit chan bool
	pendingOperations     chan bool
	client                *ssh.Client
)

func init() {
	ModifiedFiles = make(chan string, FilesBuffer)
	DeletedFiles = make(chan string, FilesBuffer)
	RemoteFileManagerExit = make(chan bool)
	pendingOperations = make(chan bool, ConcurrentOperations)
}

// RemoteFileManager manages files on the server. This function handles the
// deleting and uploading of files/directories
func RemoteFileManager() {
	for {
		select {
		case file := <-ModifiedFiles:
			if err := OpenClient(); err != nil {
				Log.Errorf("Error occurred while opening connection: %v\n", err)
				continue
			}

			Log.Infoln("Modified file:", file)

			//pendingOperations <- true
		case file := <-DeletedFiles:
			if err := OpenClient(); err != nil {
				Log.Errorf("Error occurred while opening connection: %v\n", err)
				continue
			}

			Log.Infoln("Deleted file:", file)

			pendingOperations <- true
			deleteRemoteFile(file)
		case <-time.After(ConnectionTimeout):
			// After 5 minutes of inactivity, close the connection
			CloseClient()
		case <-RemoteFileManagerExit:
			Log.Debugln("RemoteFileManagerExit signal received, wrapping up...")
			goto _cleanup
		}
	}

_cleanup:
	CloseClient()
	close(ModifiedFiles)
	close(DeletedFiles)

	for len(pendingOperations) > 0 {
		Log.Infoln("Pending operations:", len(pendingOperations))
		<-time.After(2 * time.Second)
	}

	Log.Debugln("Exiting RemoteFileManager")

	RemoteFileManagerExit <- true
}

func CloseClient() {
	if client == nil {
		return
	}

	if err := client.Close(); err != nil {
		Log.Errorf("Error occurred while closing client connection: %v\n", err)
	}

	client = nil
}

func OpenClient() (err error) {
	if client != nil {
		return nil
	}

	host := fmt.Sprintf("%s:%d", Host, 22)
	Log.Debugln("Attempting to connect to", host)

	client, err = ssh.Dial("tcp", host, Config)

	return
}

func deleteRemoteFile(name string) error {
	defer func() { <-pendingOperations }()

	sftp, err := sftp.NewClient(client)
	if err != nil {
		return err
	}

	fullPath := path.Join(RemoteRoot, name)

	Log.Infoln("Deleting", fullPath)

	return sftp.Remove(fullPath)
}
