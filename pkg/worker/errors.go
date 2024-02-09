package worker

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
	ErrBandwidthIsNil        = &Error{Code: "ErrBandwidthIsNil", Message: "Bandwidth is nil"}
	ErrPacketLossIsNil       = &Error{Code: "ErrPacketLossIsNil", Message: "PacketLoss is nil"}
	ErrLatencyIsNil          = &Error{Code: "ErrLatencyIsNil", Message: "Latency is nil"}
	ErrInitMinioClient       = &Error{Code: "ErrInitMinioClient", Message: "Failed to initialize Minio client"}
	ErrNoInterfaceFoundForIP = &Error{Code: "ErrNoInterfaceFoundForIP", Message: "No interface found for IP"}
	ErrCheckingBucketExists  = &Error{Code: "ErrCheckingBucketExists", Message: "Failed to check if bucket exists"}
	ErrCreatingBucket        = &Error{Code: "ErrCreatingBucket", Message: "Failed to create bucket"}
	ErrPushingDataToMinio    = &Error{Code: "ErrPushingDataToMinio", Message: "Failed to push data to Minio"}
	ErrGettingPresignedURL   = &Error{Code: "ErrGettingPresignedURL", Message: "Failed to get presigned URL"}
)
