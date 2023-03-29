package metrics

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"go.uber.org/zap"
)

type CostRateLimitPolicy struct {
	Logger *zap.SugaredLogger
}

func (p CostRateLimitPolicy) Do(req *policy.Request) (*http.Response, error) {
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
		p.Logger.Infof(`detected ratelimit "%v" of "%v"`, name, val)
	}
}
