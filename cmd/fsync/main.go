package main

import (
	"os"
	"os/signal"

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

	log.Debug("Starting watcher goroutine")
	go internal.Watch(watchDir)
	go internal.RemoteFileManager()

	appExit := make(chan bool)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		// c is only ever going to be an interrupt
		<-c
		internal.WatchExit <- true
		appExit <- true
	}()

	<-appExit
	log.Debug("Exiting")
}
