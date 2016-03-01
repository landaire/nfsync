package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/crypto/ssh"

	"gitlab.com/landaire/fsync/cmd/fsync/internal"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var (
	log *logrus.Logger
)

func main() {
	log = internal.Logger()

	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Enable verbose logging",
		},
		cli.StringFlag{
			Name:  "identity_file",
			Usage: "Private key file used for authentication",
		},
	}
	app.Action = watch
	app.Name = "fsync"
	app.Usage = `[<local dir>] user@remote-host:/remote/directory
	If no local dir is supplied, the working directory is assumed
	`

	app.Run(os.Args)
}

func watch(context *cli.Context) {
	argc := len(context.Args())
	if argc < 1 || argc > 2 {
		context.App.Command("help").Run(context)
		return
	}

	watchDir, err := os.Getwd()
	if err != nil {
		internal.Log.Fatalln(err)
	}

	if argc > 1 {
		watchDir = context.Args()[0]
	}

	internal.SetVerbose(context.GlobalBool("verbose"))
	setupSSHConfig(context)

	log.Debug("Starting watcher goroutine")
	go internal.Watch(watchDir)
	go internal.RemoteFileManager()

	appExit := make(chan bool)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		// c is only ever going to be an interrupt
		<-c

		chans := []chan bool{
			internal.WatchExit,
			internal.RemoteFileManagerExit,
			appExit,
		}

		for _, c := range chans {
			c <- true
		}
	}()

	<-appExit
	log.Debug("Exiting")
}

func setupSSHConfig(context *cli.Context) {
	var user, host, userAndHost string

	argc := len(context.Args())
	if argc > 1 {
		userAndHost = context.Args()[1]
	} else {
		userAndHost = context.Args()[0]
	}

	parts := strings.Split(userAndHost, "@")
	if len(parts) == 1 {
		log.Fatalln("Invalid host")
	}

	user = parts[0]
	host = parts[1]

	authMethods := []ssh.AuthMethod{}

	if ident := context.GlobalString("identity_file"); ident != "" {
		data, err := ioutil.ReadFile(ident)
		if err != nil {
			log.Fatalln("Could not read ident file:", err)
		}

		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			log.Fatalln("Could not parse ident file:", err)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		var password string
		if items, err := fmt.Scanf("Password: %s", &password); items != 1 || err != nil {
			log.Fatalln("Invalid password")
		}

		authMethods = append(authMethods, ssh.Password(password))
	}

	internal.Host = host
	internal.Config = &ssh.ClientConfig{
		User: user,
		Auth: authMethods,
	}
}
