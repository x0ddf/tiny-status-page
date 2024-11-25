package utils

import "os"

func IsRunningInCluster() bool {
	// Check if the service account token file exists
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return true
	}
	return false
}
