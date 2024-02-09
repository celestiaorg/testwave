package playbook

import (
	"github.com/celestiaorg/testwave/pkg/worker"
)

type Playbook interface {
	Name() string
	RunWorker(*worker.Worker) error
	Setup() error
	NodeSets() []*NodeSet
}

// A NodeSet has a set of worker nodes that are deployed together.
// Preferably in one pod.
type NodeSet struct {
	UID     string
	Workers []*worker.Worker
}

type SetupOpts struct {
	NodeSets []*NodeSet
}
