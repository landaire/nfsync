package internal

import "github.com/Sirupsen/logrus"

var (
	Log *logrus.Logger
)

func init() {
	Log = logrus.New()
}

func SetVerbose(verbose bool) {
	if verbose {
		Log.Level = logrus.DebugLevel
	}
}

func Logger() *logrus.Logger {
	return Log
}
