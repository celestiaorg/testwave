package testwave

import (
	"os"
	"strconv"

	"github.com/celestiaorg/testwave/pkg/message"
	"github.com/celestiaorg/testwave/pkg/playbook"
	"github.com/celestiaorg/testwave/pkg/worker"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func workerCmd(testPlanPlaybook playbook.Playbook) *cobra.Command {
	if testPlanPlaybook == nil {
		logrus.Errorf("Error: test plan playbook is nil")
		os.Exit(1)
	}

	cmd := &cobra.Command{
		Use:   WorkerCmd,
		Short: "start the worker node",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			testPlanName := os.Getenv(worker.EnvTestPlan)
			if testPlanName == "" {
				logrus.Errorf("Error reading test plan name from environment variable %s", worker.EnvTestPlan)
				os.Exit(1)
			}

			uid := os.Getenv(worker.EnvUID)
			if uid == "" {
				logrus.Errorf("Error reading worker UID from environment variable %s", worker.EnvUID)
				os.Exit(1)
			}

			rdc := prepareRedisClient()
			defer func() {
				if err := rdc.Close(); err != nil {
					logrus.Fatal("redis client close: ", err)
				}
			}()
			if _, err := rdc.Ping().Result(); err != nil {
				logrus.Error("redis ping: ", err)
				os.Exit(1)
			}

			workerNode := &worker.Worker{
				UID: uid,
				Message: &message.Message{
					NodeUID: uid,
					Client:  rdc,
				},
				Minio: prepareMinio(),
			}

			wIP, err := workerNode.LocalIPAddress()
			if err != nil {
				logrus.Errorf("Error getting local IP address: %v", err)
				os.Exit(1)
			}

			if err := workerNode.Message.SetIP(wIP); err != nil {
				logrus.Errorf("Error setting IP address: %v", err)
				os.Exit(1)
			}

			iface, err := workerNode.NetworkInterfaceByIP(wIP)
			if err != nil {
				logrus.Errorf("Error getting network interface: %v", err)
				os.Exit(1)
			}
			workerNode.BitTwister = worker.NewBitTwister(iface)

			if err := testPlanPlaybook.RunWorker(workerNode); err != nil {
				logrus.Errorf("Error running the playbook test: %v", err)
				os.Exit(1)
			}
			return nil
		},
	}
	return cmd
}

func prepareMinio() *worker.Minio {
	endpoint, ok := os.LookupEnv(worker.EnvMinioEndpoint)
	if !ok {
		logrus.Errorf("Error reading minio endpoint from environment variable %s", worker.EnvMinioEndpoint)
		os.Exit(1)
	}

	accessKey, ok := os.LookupEnv(worker.EnvMinioAccessKey)
	if !ok {
		logrus.Errorf("Error reading minio access key from environment variable %s", worker.EnvMinioAccessKey)
		os.Exit(1)
	}

	secretKey, ok := os.LookupEnv(worker.EnvMinioSecretKey)
	if !ok {
		logrus.Errorf("Error reading minio secret key from environment variable %s", worker.EnvMinioSecretKey)
		os.Exit(1)
	}

	minio, err := worker.NewMinio(worker.MinioConfig{
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
	})
	if err != nil {
		logrus.Errorf("Error creating minio client: %v", err)
		os.Exit(1)
	}
	return minio
}

func prepareRedisClient() *redis.Client {
	redisAddr, ok := os.LookupEnv(message.EnvRedisAddr)
	if !ok {
		logrus.Errorf("Error reading redis address from environment variable %s", message.EnvRedisAddr)
		os.Exit(1)
	}
	redisPassword := os.Getenv(message.EnvRedisPassword)

	redisDB, err := strconv.ParseInt(os.Getenv(message.EnvRedisDB), 10, 64)
	if err != nil {
		logrus.Info("Error reading redis DB from environment variable `%s`, defaulted to `0`: %v", message.EnvRedisDB, err)
		redisDB = 0
	}

	return redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       int(redisDB),
	})
}
