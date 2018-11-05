package reportutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Azure/adx-automation-agent/sdk/common"
	"github.com/Azure/adx-automation-agent/sdk/models"
	"github.com/sirupsen/logrus"
)

var httpClient = &http.Client{}

// Report method requests the email service to send emails
func Report(run *models.Run, receivers []string, templateURL string) {
	logrus.Info("Sending report...")

	// Emails should not be sent to all the team if the run was not set with a remark
	// Only acceptable remark for sending emails to whole team is 'official'
	if !run.IsOfficial() {
		receivers = []string{}
	}

	if email, ok := run.Settings[common.KeyUserEmail]; ok {
		receivers = append(receivers, email.(string))
	}

	if len(receivers) > 0 {
		content := make(map[string]string)
		content["run_id"] = strconv.Itoa(run.ID)
		content["receivers"] = strings.Join(receivers, ",")
		content["template"] = templateURL

		body, err := json.Marshal(content)
		if err != nil {
			logrus.Info("Fail to marshal JSON during request sending email.")
			return
		}

		logrus.Info(string(body))
		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("http://%s/report", common.DNSNameEmailService),
			bytes.NewBuffer(body))
		if err != nil {
			logrus.Info("Fail to create request to requesting email.")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			logrus.Info("Fail to send request to email service.")
			return
		}

		if resp.StatusCode != http.StatusOK {
			logrus.Info("The request may have failed.")
		}
	} else {
		logrus.Info("Skip sending report")
	}
}
