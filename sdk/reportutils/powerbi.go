package reportutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Azure/adx-automation-agent/sdk/common"
	"github.com/Azure/adx-automation-agent/sdk/models"
	"github.com/sirupsen/logrus"
)

// RefreshPowerBI requests the PowerBI service to refresh a dataset
func RefreshPowerBI(run *models.Run, product string) {
	if !run.IsOfficial() {
		logrus.Info("Skip PowerBI refresh: run is not official")
		return
	}
	logrus.Info("sending PowerBI refresh request...")

	content := map[string]interface{}{
		"product": product,
		"runID":   run.ID,
	}
	body, err := json.Marshal(content)
	if err != nil {
		logrus.Info("Fail to marshal JSON during request refreshing PowerBI.")
		return
	}

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://%s/report", common.DNSNameReportService),
		bytes.NewBuffer(body))
	if err != nil {
		logrus.Info(fmt.Sprintf("Fail to create request to refresh PowerBI: %v", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		logrus.Info(fmt.Sprintf("Fail to send request to PowerBI service: %v", err))
		return
	}

	if resp.StatusCode != http.StatusOK {
		logrus.Info(fmt.Sprintf("The request may have failed. Status code: %d", resp.StatusCode))
		return
	}
	logrus.Info("Finished sending PowerBI refresh request")
}
