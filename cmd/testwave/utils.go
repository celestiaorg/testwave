package testwave

import (
	"fmt"
	"time"

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

func timeAgo(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration.Seconds() < 60:
		return fmt.Sprintf("%.0f seconds ago", duration.Seconds())
	case duration.Minutes() < 60:
		return fmt.Sprintf("%.0f minutes ago", duration.Minutes())
	case duration.Hours() < 24:
		return fmt.Sprintf("%.0f hours ago", duration.Hours())
	default:
		return fmt.Sprintf("%.0f days ago", duration.Hours()/24)
	}
}
