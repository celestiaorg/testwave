package deployer

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
	ErrUUIDGeneration             = &Error{Code: "UUIDGenerationError", Message: "Error generating UUID"}
	ErrGettingDispatcherPodConfig = &Error{Code: "GettingDispatcherPodConfigError", Message: "Error getting dispatcher pod config"}
	ErrCreateDispatcherPod        = &Error{Code: "CreateDispatcherPodError", Message: "Error creating dispatcher pod"}
	ErrGetUserHomeDir             = &Error{Code: "GetUserHomeDirError", Message: "Error getting user home directory"}
	ErrBuildingKubeconfig         = &Error{Code: "BuildingKubeconfigError", Message: "Error building kubeconfig"}
	ErrGettingContainerLogs       = &Error{Code: "GettingContainerLogsError", Message: "Error getting container logs"}
	ErrAddFilesToPod              = &Error{Code: "AddFilesToPodError", Message: "Error adding files to pod"}
)
