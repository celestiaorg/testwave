package dispatcher

import "fmt"

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Wrap(err error) error {
	e.Err = err
	return e
}

var (
	ErrNilNodeSet                    = &Error{Code: "NilNodeSet", Message: "nil nodeset"}
	ErrNoWorkersInNodeSet            = &Error{Code: "NoWorkersInNodeSet", Message: "no workers in nodeset"}
	ErrAddingFileToPod               = &Error{Code: "AddingFileToPod", Message: "error adding file to pod"}
	ErrGettingDeployment             = &Error{Code: "GettingDeployment", Message: "error getting deployment"}
	ErrCreatingDeployment            = &Error{Code: "CreatingDeployment", Message: "error creating deployment"}
	ErrCreatingAutoScaler            = &Error{Code: "CreatingAutoScaler", Message: "error creating autoscaler"}
	ErrWaitingForRedis               = &Error{Code: "WaitingForRedis", Message: "error waiting for redis"}
	ErrWaitingForMinio               = &Error{Code: "WaitingForMinio", Message: "error waiting for minio"}
	ErrCreatingService               = &Error{Code: "CreatingService", Message: "error creating service"}
	ErrGettingService                = &Error{Code: "GettingService", Message: "error getting service"}
	ErrWaitingForRedisService        = &Error{Code: "WaitingForRedisService", Message: "error waiting for redis service"}
	ErrWaitingForMinioService        = &Error{Code: "WaitingForMinioService", Message: "error waiting for minio service"}
	ErrTimeout                       = &Error{Code: "Timeout", Message: "timeout"}
	ErrFailedConnection              = &Error{Code: "FailedConnection", Message: "failed connection"}
	ErrLoadBalancerIPNotAvailable    = &Error{Code: "LoadBalancerIPNotAvailable", Message: "load balancer IP not available"}
	ErrGettingNodes                  = &Error{Code: "GettingNodes", Message: "error getting nodes"}
	ErrNoNodesFound                  = &Error{Code: "NoNodesFound", Message: "no nodes found"}
	ErrGettingMinioEndpoint          = &Error{Code: "GettingMinioEndpoint", Message: "error getting minio endpoint"}
	ErrParsingStorageSize            = &Error{Code: "ParsingStorageSize", Message: "error parsing storage size"}
	ErrListingPersistentVolumes      = &Error{Code: "ListingPersistentVolumes", Message: "error listing persistent volumes"}
	ErrCreatingPersistentVolume      = &Error{Code: "CreatingPersistentVolume", Message: "error creating persistent volume"}
	ErrCreatingPersistentVolumeClaim = &Error{Code: "CreatingPersistentVolumeClaim", Message: "error creating persistent volume claim"}
)
