package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

			pendingOperations <- true
			go func() {
				defer func() { <-pendingOperations }()
				stat, err := os.Stat(file)
				if err != nil {
					Log.Errorln(err)
					return
				}

				if stat.IsDir() {
					err = remoteMkdir(filepath.Join(RemoteRoot, file))
				} else {
					err = uploadFile(file)
				}

				if err != nil {
					Log.Errorln(err)
				}
			}()
		case file := <-DeletedFiles:
			if err := OpenClient(); err != nil {
				Log.Errorf("Error occurred while opening connection: %v\n", err)
				continue
			}

			Log.Infoln("Deleted file:", file)

			pendingOperations <- true
			go func() {
				defer func() { <-pendingOperations }()
				if err := deleteRemoteFile(file); err != nil {
					Log.Errorln(err)
				}
			}()
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
	sftp, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftp.Close()

	fullPath := filepath.Join(RemoteRoot, name)

	Log.Infoln("Deleting", fullPath)
	remoteInfo, err := sftp.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	localInfo, err := os.Stat(name)
	if err != nil {
		return err
	}

	if localInfo.IsDir() != remoteInfo.IsDir() {
		local := "is"
		remote := "is not"
		if !localInfo.IsDir() {
			local = "is not"
			remote = "is"
		}

		return fmt.Errorf("%s %s a directory and the remote file %s -- delete canceled",
			name,
			local,
			remote)
	}

	return sftp.Remove(fullPath)
}

func uploadFile(name string) error {
	sftp, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftp.Close()

	fullPath := filepath.Join(RemoteRoot, name)

	dir, _ := filepath.Split(fullPath)
	if err := remoteMkdir(dir); err != nil {
		return err
	}

	Log.Infof("Uploading %s to %s\n", name, fullPath)

	remoteFile, err := sftp.Create(fullPath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	localFile, err := os.Open(name)
	defer localFile.Close()

	written, err := io.Copy(remoteFile, localFile)
	Log.Infof("Wrote %d bytes\n", written)

	return err
}

func remoteMkdir(fullPath string) error {
	sftp, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftp.Close()

	parts := strings.Split(fullPath, string(filepath.Separator))
	partialPath := "/"

	for _, part := range parts {
		partialPath = filepath.Join(partialPath, part)

		stat, err := sftp.Lstat(partialPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}

			Log.Infoln("Creating directory", partialPath)
			if err := sftp.Mkdir(partialPath); err != nil {
				return err
			}
		} else {
			if stat.Mode()&os.ModeSymlink == os.ModeSymlink {
				continue
			} else if !stat.IsDir() {
				return fmt.Errorf("%s is not a directory", partialPath)
			}
		}
	}

	return nil
}

func checkPath(fullPath string) error {
	absolute, err := filepath.Abs(fullPath)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(absolute, RemoteRoot) {
		return fmt.Errorf("Path %s is outside RemoteRoot", absolute)
	}

	return nil
}
