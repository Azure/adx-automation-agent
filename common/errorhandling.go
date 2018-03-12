package common

import (
	"fmt"
	"log"
)

// ExitOnError exists the current program when an error presents
func ExitOnError(err error, message string) {
	if err == nil {
		return
	}

	log.Fatalf("%s: %s", message, err)
}

// PanicOnError exists the current program and sent panic message
func PanicOnError(err error, message string) {
	if err == nil {
		return
	}

	log.Fatalf("%s: %s", message, err)
	panic(fmt.Errorf("%s: %s", message, err))
}
