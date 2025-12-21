package metrics

import (
	"log/slog"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type CostRateLimitPolicy struct {
	Logger *slog.Logger
}

func (p CostRateLimitPolicy) Do(req *policy.Request) (*http.Response, error) {
	p.Logger.Debug("sending cost query")
	// Forward the request to the next policy in the pipeline.
	resp, err := req.Next()

	if resp != nil {
		p.checkFoRateLimit("costmanagement-qpu-consumed", "x-ms-ratelimit-microsoft.costmanagement-qpu-consumed", resp)
		p.checkFoRateLimit("costmanagement-qpu-remaining", "x-ms-ratelimit-microsoft.costmanagement-qpu-remaining", resp)
		p.checkFoRateLimit("costmanagement-entity-requests", "x-ms-ratelimit-remaining-microsoft.costmanagement-entity-requests", resp)
		p.checkFoRateLimit("costmanagement-tenant-requests", "x-ms-ratelimit-remaining-microsoft.costmanagement-tenant-requests", resp)
		p.checkFoRateLimit("consumption-tenant-requests", "x-ms-ratelimit-remaining-microsoft.consumption-tenant-requests", resp)
	}

	return resp, err
}

func (p CostRateLimitPolicy) checkFoRateLimit(name, header string, resp *http.Response) {
	if val := resp.Header.Get(header); val != "" {
		p.Logger.Debug(`detected ratelimit`, slog.String("name", name), slog.String("value", val))
	}
}
