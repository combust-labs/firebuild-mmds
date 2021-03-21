package mmds

import (
	"encoding/json"
	"net/http"

	"github.com/hashicorp/go-hclog"
)

// GuestFetchMMDSMetadata resolves the metadata from MMDS as a guest.
func GuestFetchMMDSMetadata(logger hclog.Logger, baseURI string) (*MMDSData, error) {
	httpRequest, err := http.NewRequest(http.MethodGet, baseURI, nil)
	if err != nil {
		logger.Error("error when creating a http request", "reason", err.Error())
		return nil, err
	}
	httpRequest.Header.Add("accept", "application/json")
	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		logger.Error("error executing MMDS request", "reason", err.Error())
		return nil, err
	}
	defer httpResponse.Body.Close()
	mmdsData := &MMDSData{}
	if err := json.NewDecoder(httpResponse.Body).Decode(mmdsData); err != nil {
		logger.Error("error deserializing MMDS data", "reason", err.Error())
		return nil, err
	}
	return mmdsData, nil
}
