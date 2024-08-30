package utils

import (
	"errors"
	"fmt"
)

func IntToBool(value int) bool {
	switch value {
	case 0:
		return false
	case 1:
		return true
	default:
		panic(errors.New(fmt.Sprintf("wrong value passed to IntToBool %d", value)))
	}
}
