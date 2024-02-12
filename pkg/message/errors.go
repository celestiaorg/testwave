package message

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
	ErrSetGlobal     = &Error{Code: "ErrSetGlobal", Message: "Error setting global message"}
	ErrPublish       = &Error{Code: "ErrPublish", Message: "Error publishing a message"}
	ErrSetIP         = &Error{Code: "ErrSetIP", Message: "Error setting IP"}
	ErrGetIP         = &Error{Code: "ErrGetIP", Message: "Error getting IP"}
	ErrMsgNotFound   = &Error{Code: "ErrMsgNotFound", Message: "Message not found"}
	ErrGetPubResult  = &Error{Code: "ErrGetPubResult", Message: "Error getting publish result"}
	ErrNoSubscribers = &Error{Code: "ErrNoSubscribers", Message: "No subscribers"}
	ErrTypeCasting   = &Error{Code: "ErrTypeCasting", Message: "Type casting failed"}
	ErrCtxDone       = &Error{Code: "ErrCtxDone", Message: "Context Done"}
)
