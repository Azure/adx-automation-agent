package reportutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Azure/adx-automation-agent/common"
	"github.com/Azure/adx-automation-agent/models"
)

// RefreshPowerBI requests the PowerBI service to refresh a dataset
func RefreshPowerBI(run *models.Run) {
	if run.IsOfficial() {
		common.LogInfo("sending PowerBI refresh request...")

		content := map[string]string{
			"product": run.GetSecretName(droidMetadata),
		}
		body, err := json.Marshal(content)
		if err != nil {
			common.LogInfo("Fail to marshal JSON during request refreshing PowerBI.")
			return
		}

		req, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("http://%s/powerbi", common.DNSNamePowerBIService),
			bytes.NewBuffer(body))
		if err != nil {
			common.LogInfo(fmt.Sprintf("Fail to create request to refresh PowerBI: %v", err))
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			common.LogInfo("Fail to send request to PowerBI service.")
			return
		}

		if resp.StatusCode != http.StatusOK {
			common.LogInfo("The request may have failed.")
			return
		}

		common.LogInfo("Finished sending PowerBI refresh request")
	}
	common.LogInfo("Skip PowerBI refresh: run is not official")
}
