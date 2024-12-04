package logging

import (
	"fmt"
)

type InvalidLevelError struct {
	Message string
}

func (e *InvalidLevelError) Error() string {
	return fmt.Sprintf("Invalid logging level: %s", e.Message)
}
