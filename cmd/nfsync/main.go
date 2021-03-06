package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"gitlab.com/landaire/nfsync/cmd/nfsync/internal"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/howeyc/gopass"
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
			Name:  "identity_file, i",
			Usage: "Private key file used for authentication",
		},
		cli.BoolFlag{
			Name:  "password",
			Usage: "Use password authentication",
		},
	}
	app.Action = watch
	app.Name = "nfsync"
	app.Usage = "sync local file system changes with a remote server"

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

	log.Debugln("Starting watcher goroutine with watchdir", watchDir)
	go internal.Watch(watchDir)
	log.Debugln("Starting remote file manager goroutine")
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
	<-internal.RemoteFileManagerExit
	<-internal.WatchExit
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

	hostParts := strings.Split(userAndHost, "@")
	if len(hostParts) != 2 {
		log.Fatalln("Invalid host")
	}

	user = hostParts[0]

	hostDirParts := strings.Split(hostParts[1], ":")
	if len(hostDirParts) != 2 {
		log.Fatalln("Invalid remote dir specified")
	}

	host = hostDirParts[0]
	startDir := hostDirParts[1]

	authMethods := []ssh.AuthMethod{}

	if sshAgentAuthMethod := sshAgent(); sshAgentAuthMethod != nil {
		log.Debugln("Adding ssh agent auth")
		//authMethods = append(authMethods, sshAgentAuthMethod)
	}

	if ident := context.GlobalString("identity_file"); ident != "" {
		log.Debugln("Using identity file:", ident)
		data, err := ioutil.ReadFile(ident)
		if err != nil {
			log.Fatalln("Could not read ident file:", err)
		}

		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			log.Fatalln("Could not parse ident file:", err)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	authMethods = append(authMethods, ssh.KeyboardInteractive(interactiveAuth))

	internal.RemoteRoot = startDir
	internal.Host = host
	internal.Config = &ssh.ClientConfig{
		User: user,
		Auth: authMethods,
	}

	log.Debugln("RemoteRoot:", startDir)
	log.Debugln("Host:", host)
	log.Debugln("ssh config:", internal.Config)

	if err := internal.OpenClient(); err != nil {
		log.Fatalln(err)
	}
}

func sshAgent() ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}

	return nil
}

func interactiveAuth(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
	log.Debugln("Using interactive auth")

	if user != "" {
		fmt.Println(user)
	}
	if instruction != "" {
		fmt.Println(instruction)
	}

	for i, question := range questions {
		var answer string
		fmt.Print(question)

		if echos[i] {
			fmt.Scanln("%s", &answer)
		} else {
			password, err := gopass.GetPasswd()

			if err != nil {
				return answers, err
			}

			answer = string(password)
		}

		answers = append(answers, answer)
	}

	return
}
