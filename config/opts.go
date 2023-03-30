package config

import (
	"encoding/json"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug       bool `long:"log.debug"    env:"LOG_DEBUG"  description:"debug mode"`
			Development bool `long:"log.devel"    env:"LOG_DEVEL"  description:"development mode"`
			Json        bool `long:"log.json"     env:"LOG_JSON"   description:"Switch log output to json format"`
		}

		// azure
		Azure struct {
			Tenant            *string  `long:"azure.tenant"                   env:"AZURE_TENANT_ID"           description:"Azure tenant id" required:"true"`
			Environment       *string  `long:"azure.environment"              env:"AZURE_ENVIRONMENT"         description:"Azure environment name" default:"AZUREPUBLICCLOUD"`
			Subscription      []string `long:"azure.subscription"             env:"AZURE_SUBSCRIPTION_ID"     env-delim:" "  description:"Azure subscription ID (space delimiter)"`
			Location          []string `long:"azure.location"                 env:"AZURE_LOCATION"            env-delim:" "  description:"Azure locations (space delimiter)"                                  default:"westeurope" default:"northeurope"` //nolint:staticcheck
			ResourceGroupTags []string `long:"azure.resourcegroup.tag"        env:"AZURE_RESOURCEGROUP_TAG"   env-delim:" "  description:"Azure ResourceGroup tags (space delimiter)"`
			ResourceTags      []string `long:"azure.resource.tag"             env:"AZURE_RESOURCE_TAG"        env-delim:" "  description:"Azure Resource tags (space delimiter)"`
		}

		// scrape times
		Scrape struct {
			Time struct {
				Default        time.Duration  `long:"scrape.time"                    env:"SCRAPE_TIME"                    description:"Default scrape time (time.duration)"                      default:"5m"`
				Exporter       *time.Duration `long:"scrape.time.exporter"           env:"SCRAPE_TIME_EXPORTER"           description:"Scrape time for exporter metrics (time.duration)"         default:"10s"`
				General        *time.Duration `long:"scrape.time.general"            env:"SCRAPE_TIME_GENERAL"            description:"Scrape time for general metrics (time.duration)"`
				Resource       *time.Duration `long:"scrape.time.resource"           env:"SCRAPE_TIME_RESOURCE"           description:"Scrape time for resource metrics  (time.duration)"`
				Quota          *time.Duration `long:"scrape.time.quota"              env:"SCRAPE_TIME_QUOTA"              description:"Scrape time for quota metrics  (time.duration)"`
				Defender       *time.Duration `long:"scrape.time.defender"           env:"SCRAPE_TIME_DEFENDER"           description:"Scrape time for Defender metrics (time.duration)"`
				ResourceHealth *time.Duration `long:"scrape.time.resourcehealth"     env:"SCRAPE_TIME_RESOURCEHEALTH"     description:"Scrape time for ResourceHealth metrics (time.duration)"`
				Iam            *time.Duration `long:"scrape.time.iam"                env:"SCRAPE_TIME_IAM"                description:"Scrape time for IAM metrics (time.duration)"`
				Graph          *time.Duration `long:"scrape.time.graph"              env:"SCRAPE_TIME_GRAPH"              description:"Scrape time for Graph metrics (time.duration)"`
				Costs          *time.Duration `long:"scrape.time.costs"              env:"SCRAPE_TIME_COSTS"              description:"Scrape time for costs/consumtion metrics (time.duration; BETA)" default:"0"`
				Portscan       *time.Duration `long:"scrape.time.portscan"           env:"SCRAPE_TIME_PORTSCAN"           description:"Scrape time for public ips for portscan (time.duration)"`
			}
		}

		ResourceHealth struct {
			SummaryMaxLength int `long:"resourcehealth.summary.maxlength"           env:"RESOURCEHEALTH_SUMMARY_MAXLENGTH"  description:"Max length of ResourceHealth summary label (0 = disable summary label)"  default:"0"`
		}

		// graph settings
		Graph struct {
			ApplicationFilter      string `long:"graph.application.filter"       env:"GRAPH_APPLICATION_FILTER"       description:"MS Graph application $filter query eg: startswith(displayName,'A')"`
			ServicePrincipalFilter string `long:"graph.serviceprincipal.filter"  env:"GRAPH_SERVICEPRINCIPAL_FILTER"  description:"MS Graph serviceprincipal $filter query eg: startswith(displayName,'A')"`
		}

		// costs
		Costs struct {
			Timeframe    []string      `long:"costs.timeframe"     env:"COSTS_TIMEFRAME"  env-delim:" " description:"Timeframe for cost reportings  (space delimiter)" default:"MonthToDate" default:"YearToDate"` //nolint:staticcheck
			Queries      []string      `long:"costs.query"                                              description:"Cost query in format: 'queryname=dimension' or 'queryname=dimension1,dimension2,dimension3'. Dimensions can be: 'ResourceGroupName','ResourceLocation','ConsumedService','ResourceType','ResourceId','MeterId','BillingMonth','MeterCategory','MeterSubcategory','Meter','AccountName','DepartmentName','SubscriptionId','SubscriptionName','ServiceName','ServiceTier','EnrollmentAccountName','BillingAccountId','ResourceGuid','BillingPeriod','InvoiceNumber','ChargeType','PublisherType','ReservationId','ReservationName','Frequency','PartNumber','CostAllocationRuleName','MarkupRuleName','PricingModel'. Can be specified in env vars as COSTS_QUERY_queryname=Dimensions"`
			ValueField   string        `long:"costs.value"                                              description:"Value field for cost query" choice:"Cost" choice:"PreTaxCost" default:"PreTaxCost"` //nolint:staticcheck
			RequestDelay time.Duration `long:"costs.request.delay" env:"COSTS_REQUEST_DELAY" description:"Delay API requests by this time to avoid ratelimits" default:"10s"`
		}

		// portscan settings
		Portscan struct {
			Enabled   bool          `long:"portscan"                      env:"PORTSCAN"                                 description:"Enable portscan for public IPs"`
			Time      time.Duration `long:"portscan.time"                 env:"PORTSCAN_TIME"                            description:"Portscan time (time.duration)"                         default:"3h"`
			Parallel  int           `long:"portscan.parallel"             env:"PORTSCAN_PARALLEL"                        description:"Portscan parallel scans (parallel * threads = concurrent gofuncs)"  default:"2"`
			Threads   int           `long:"portscan.threads"              env:"PORTSCAN_THREADS"                         description:"Portscan threads (concurrent port scans per IP)"  default:"1000"`
			Timeout   int           `long:"portscan.timeout"              env:"PORTSCAN_TIMEOUT"                         description:"Portscan timeout (seconds)"                       default:"5"`
			PortRange []string      `long:"portscan.range"                env:"PORTSCAN_RANGE"            env-delim:" "  description:"Portscan port range (first-last)  (space delimiter)"                 default:"1-65535"`
		}

		// caching
		Cache struct {
			Path string `long:"cache.path" env:"CACHE_PATH" description:"Cache path (to folder, file://path... or azblob://storageaccount.blob.core.windows.net/containername)"`
		}

		Server struct {
			// general options
			Bind         string        `long:"server.bind"              env:"SERVER_BIND"           description:"Server address"        default:":8080"`
			ReadTimeout  time.Duration `long:"server.timeout.read"      env:"SERVER_TIMEOUT_READ"   description:"Server read timeout"   default:"5s"`
			WriteTimeout time.Duration `long:"server.timeout.write"     env:"SERVER_TIMEOUT_WRITE"  description:"Server write timeout"  default:"10s"`
		}
	}
)

func (o *Opts) GetCachePath(path string) (ret *string) {
	if o.Cache.Path != "" {
		tmp := o.Cache.Path + "/" + path
		ret = &tmp
	}

	return
}

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}
