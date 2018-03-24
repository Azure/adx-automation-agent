package common

import "log"

// LogInfo writes information to stdout
func LogInfo(message string) {
	log.Printf("INFO: %s", message)
}

// LogWarning writes warning to stdout
func LogWarning(message string) {
	log.Printf("WARN: %s", message)
}

// LogError writes warning to stdout
func LogError(message string) {
	log.Printf("ERRO: %s", message)
}
