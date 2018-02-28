package httputils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/kubeutils"
)

// GetEndpointFromEnv returns the endpoint defined in the environment
func GetEndpointFromEnv() (endpoint string, err error) {
	if endpoint, ok := os.LookupEnv(common.EnvKeyStoreName); ok {
		if kubeutils.IsInCluster() {
			return fmt.Sprintf("http://%s", endpoint), nil
		}
		return fmt.Sprintf("https://%s", endpoint), nil
	}

	return "", fmt.Errorf("Environment variable %s is not defined", common.EnvKeyStoreName)
}

// CreateRequest returns a new HTTP request
func CreateRequest(method string, path string, body []byte) (request *http.Request, err error) {
	templateError := fmt.Sprintf("Fail to create request [%s %s].", method, path) + " Reason %s. Exception %s."
	authorizationHeader := os.Getenv(common.EnvKeyInternalCommunicationKey)
	endpoint, err := GetEndpointFromEnv()
	if err != nil {
		return nil, err
	}

	var buffer io.Reader
	if body != nil {
		buffer = bytes.NewBuffer(body)
	}

	request, err = http.NewRequest(method, fmt.Sprintf("%s/%s", endpoint, path), buffer)
	if err != nil {
		return nil, fmt.Errorf(templateError, "unable to create request", err)
	}

	request.Header.Set("Authorization", authorizationHeader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	return
}
