package httputils

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/Azure/adx-automation-agent/sdk/common"
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
	authorizationHeader := os.Getenv(common.EnvKeyInternalCommunicationKey)
	endpoint := getEndpointFromEnv()

	var buffer io.Reader
	if body != nil {
		buffer = bytes.NewBuffer(body)
	}

	request, err = http.NewRequest(method, fmt.Sprintf("%s/%s", endpoint, path), buffer)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %s", err.Error())
	}

	request.Header.Set("Authorization", authorizationHeader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	return
}

// SendRequest sends the given request and verify the response's status code
func SendRequest(request *http.Request) ([]byte, error) {
	httpClient := http.Client{CheckRedirect: nil}
	resp, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("http error: %s", err.Error())
	}

	defer resp.Body.Close()
	respContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %s", err.Error())
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP Status %d: %s", resp.StatusCode, string(respContent))
	}

	return respContent, nil
}
