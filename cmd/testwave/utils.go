package testwave

import (
	"github.com/sirupsen/logrus"
)

func printUnQuotedLogs(logs string) {
	qStatus := logrus.TextFormatter{}.DisableQuote
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableQuote: true,
	})
	logrus.Info(logs)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableQuote: qStatus,
	})
}
