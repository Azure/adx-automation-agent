package httputils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Azure/adx-automation-agent/common"
)

// getEndpointFromEnv returns the endpoint defined in the environment
func getEndpointFromEnv() string {
	if endpoint, ok := os.LookupEnv(common.EnvKeyStoreName); ok {
		return endpoint
	}

	return fmt.Sprintf("http://%s/api", common.DNSNameTaskStore)
}

// CreateRequest returns a new HTTP request
func CreateRequest(method string, path string, body []byte) (request *http.Request, err error) {
	templateError := fmt.Sprintf("Fail to create request [%s %s].", method, path) + " Reason %s. Exception %s."
	authorizationHeader := os.Getenv(common.EnvKeyInternalCommunicationKey)
	endpoint := getEndpointFromEnv()

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
