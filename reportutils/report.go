package reportutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/models"
)

var httpClient = &http.Client{}

// Report method requests the email service to send emails
func Report(run *models.Run) {
	common.LogInfo("Sending report...")
	if email, ok := run.Settings[common.KeyUserEmail]; ok {
		content := make(map[string]string)
		content["run_id"] = strconv.Itoa(run.ID)
		content["receivers"] = email.(string)

		body, err := json.Marshal(content)
		if err != nil {
			common.LogInfo("Fail to marshal JSON during request sending email.")
			return
		}

		common.LogInfo(string(body))
		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("http://%s/report", common.DNSNameEmailService),
			bytes.NewBuffer(body))
		if err != nil {
			common.LogInfo("Fail to create request to requesting email.")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			common.LogInfo("Fail to send request to email service.")
			return
		}

		if resp.StatusCode != http.StatusOK {
			common.LogInfo("The request may have failed.")
		}
	} else {
		common.LogInfo("Skip sending report")
	}
}
