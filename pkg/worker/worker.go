package worker

import (
	"fmt"
	"net"

	"github.com/celestiaorg/testwave/pkg/message"
)

const (
	Cmd = "worker"

	EnvNodeType  = "WORKER_NODE_TYPE"
	EnvWorkerUID = "WORKER_UID"
	EnvPodUID    = "POD_UID"
	EnvTestPlan  = "TEST_PLAN"
	EnvNamespace = "NAMESPACE" //k8s namespace

	ipFinderProtocol = "udp"
	ipFinderAddress  = "8.8.8.8:80"
)

// Worker represents a node with one controller and multiple app containers.
type Worker struct {
	UID        string
	Message    *message.Message
	BitTwister *BitTwister
	Minio      *Minio
	Envs       map[string]string
	Files      map[string]string // map[source_file_path]mapped_file_path
}

func (w *Worker) LocalIPAddress() (string, error) {
	conn, err := net.Dial(ipFinderProtocol, ipFinderAddress)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func (w *Worker) NetworkInterfaceByIP(ipAddr string) (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				return nil, err
			}

			if ip.String() == ipAddr {
				return &iface, nil
			}
		}
	}

	return nil, ErrNoInterfaceFoundForIP.Wrap(fmt.Errorf("address: %s", ipAddr))
}
