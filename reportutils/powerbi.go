package reportutils

import (
	"fmt"
	"net/http"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/kubeutils"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// RefreshPowerBI refreshes a PowerBI dataset. It takes the PBI group and
// dataset parameters from the product's secret.
// See https://powerbi.microsoft.com/en-us/blog/announcing-data-refresh-apis-in-the-power-bi-service/
func RefreshPowerBI(secret *corev1.Secret) {
	common.LogInfo("sending PowerBI refresh request...")
	// get parameters
	skipTemplate := "Secret does not have a `%s` key. Skipping Power BI refresh"
	groupKey := "powerbi.group"
	datasetKey := "powerbi.dataset"

	group, ok := secret.Data[groupKey]
	if !ok {
		common.LogInfo(fmt.Sprintf(skipTemplate, groupKey))
		return
	}
	dataset, ok := secret.Data[datasetKey]
	if !ok {
		common.LogInfo(fmt.Sprintf(skipTemplate, datasetKey))
		return
	}

	// get client and authorizer
	client := autorest.NewClientWithUserAgent("")
	authorizer, err := getAuth()
	if err != nil {
		common.LogInfo(fmt.Sprintf("failed to get authorizer: %v", err))
		return
	}
	client.Authorizer = authorizer

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("https://api.powerbi.com/v1.0/myorg/groups/%s/datasets/%s/refreshes", string(group), string(dataset)),
		nil)
	if err != nil {
		common.LogInfo(fmt.Sprintf("failed to create request: %v", err))
		return
	}

	resp, err := autorest.SendWithSender(client, req,
		autorest.DoRetryForStatusCodes(client.RetryAttempts, client.RetryDuration, autorest.StatusCodesForRetry...))
	if err != nil {
		common.LogInfo(fmt.Sprintf("failed to send refresh request: %v", err))
		return
	}

	err = autorest.Respond(
		resp,
		client.ByInspecting(),
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusAccepted),
		autorest.ByClosing())
	if err != nil {
		common.LogInfo(fmt.Sprintf("failed to respond to PowerBI refresh response: %v", err))
		return
	}
	common.LogInfo("finished sending PowerBI refresh request")
}

func getAuth() (a autorest.Authorizer, err error) {
	secret, err := kubeutils.TryCreateKubeClientset().
		CoreV1().
		Secrets(common.GetCurrentNamespace("a01-prod")).
		Get("email", metav1.GetOptions{})
	if err != nil {
		return a, fmt.Errorf("failed to get the kubernetes secret: %v", err)
	}
	clientID := secret.Data["powerbi.client.id"]
	username := secret.Data["username"]
	password := secret.Data["password"]

	endpoint, err := url.Parse("https://login.windows.net/common/oauth2/token")
	config := adal.OAuthConfig{
		TokenEndpoint: *endpoint,
	}
	if err != nil {
		return a, fmt.Errorf("failed to parse token endpoint: %v", err)
	}
	spt, err := adal.NewServicePrincipalTokenFromUsernamePassword(
		config,
		string(clientID),
		string(username),
		string(password),
		"https://analysis.windows.net/powerbi/api")
	if err != nil {
		return a, fmt.Errorf("failed to create a new service principal token: %v", err)
	}
	return autorest.NewBearerAuthorizer(spt), nil
}
