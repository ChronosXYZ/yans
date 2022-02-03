package protocol

import "fmt"

type NNTPResponse struct {
	Code    int
	Message string
}

func (nr NNTPResponse) String() string {
	return fmt.Sprintf("%d %s", nr.Code, nr.Message)
}
