package config

import (
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug   bool `           long:"debug"        env:"DEBUG"    description:"debug mode"`
			Verbose bool `short:"v"  long:"verbose"      env:"VERBOSE"  description:"verbose mode"`
			LogJson bool `           long:"log.json"     env:"LOG_JSON" description:"Switch log output to json format"`
		}

		// azure
		Azure struct {
			Tenant            *string  `long:"azure-tenant"                   env:"AZURE_TENANT_ID"           description:"Azure tenant id" required:"true"`
			Environment       *string  `long:"azure-environment"              env:"AZURE_ENVIRONMENT"         description:"Azure environment name" default:"AZUREPUBLICCLOUD"`
			Subscription      []string `long:"azure-subscription"             env:"AZURE_SUBSCRIPTION_ID"     env-delim:" "  description:"Azure subscription ID (space delimiter)"`
			Location          []string `long:"azure-location"                 env:"AZURE_LOCATION"            env-delim:" "  description:"Azure locations (space delimiter)"                                  default:"westeurope" default:"northeurope"` //nolint:staticcheck
			ResourceGroupTags []string `long:"azure-resourcegroup-tag"        env:"AZURE_RESOURCEGROUP_TAG"   env-delim:" "  description:"Azure ResourceGroup tags (space delimiter)"                         default:"owner"`
			ResourceTags      []string `long:"azure-resource-tag"             env:"AZURE_RESOURCE_TAG"        env-delim:" "  description:"Azure Resource tags (space delimiter)"                              default:"owner"`
		}

		// scrape times
		Scrape struct {
			Time               time.Duration  `long:"scrape-time"                    env:"SCRAPE_TIME"                    description:"Default scrape time (time.duration)"                      default:"5m"`
			TimeRateLimitRead  *time.Duration `long:"scrape-ratelimit-read"          env:"SCRAPE_RATELIMIT_READ"          description:"Scrape time for ratelimit read metrics (time.duration)"   default:"2m"`
			TimeRateLimitWrite *time.Duration `long:"scrape-ratelimit-write"         env:"SCRAPE_RATELIMIT_WRITE"         description:"Scrape time for ratelimit write metrics (time.duration)"  default:"5m"`
			TimeExporter       *time.Duration `long:"scrape-time-exporter"           env:"SCRAPE_TIME_EXPORTER"           description:"Scrape time for exporter metrics (time.duration)"         default:"10s"`
			TimeGeneral        *time.Duration `long:"scrape-time-general"            env:"SCRAPE_TIME_GENERAL"            description:"Scrape time for general metrics (time.duration)"`
			TimeResource       *time.Duration `long:"scrape-time-resource"           env:"SCRAPE_TIME_RESOURCE"           description:"Scrape time for resource metrics  (time.duration)"`
			TimeQuota          *time.Duration `long:"scrape-time-quota"              env:"SCRAPE_TIME_QUOTA"              description:"Scrape time for quota metrics  (time.duration)"`
			TimeSecurity       *time.Duration `long:"scrape-time-security"           env:"SCRAPE_TIME_SECURITY"           description:"Scrape time for Security metrics (time.duration)"`
			TimeResourceHealth *time.Duration `long:"scrape-time-resourcehealth"     env:"SCRAPE_TIME_RESOURCEHEALTH"     description:"Scrape time for ResourceHealth metrics (time.duration)"`
			TimeIam            *time.Duration `long:"scrape-time-iam"                env:"SCRAPE_TIME_IAM"                description:"Scrape time for IAM metrics (time.duration)"`
			TimeGraph          *time.Duration `long:"scrape-time-graph"              env:"SCRAPE_TIME_GRAPH"              description:"Scrape time for Graph metrics (time.duration)"`
			TimeCosts          *time.Duration `long:"scrape-time-costs"              env:"SCRAPE_TIME_COSTS"              description:"Scrape time for costs/consumtion metrics (time.duration; BETA)" default:"0"`
		}

		// graph settings
		Graph struct {
			ApplicationFilter string `long:"graph-application-filter"    env:"GRAPH_APPLICATION_FILTER"               description:"Graph application filter query eg: startswith(displayName,'A')"`
		}

		// costs

		Costs struct {
			Timeframe []string `long:"costs-timeframe" env:"COSTS_TIMEFRAME"  env-delim:" " description:"Timeframe for cost reportings  (space delimiter)" default:"MonthToDate" default:"YearToDate"`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       //nolint:staticcheck
			Dimension []string `long:"costs-dimension" env:"COSTS_DIMENSION"  env-delim:" " description:"Dimensions for detailed cost metrics (eg 'ResourceGroup','ResourceGroupName','ResourceLocation','ConsumedService','ResourceType','ResourceId','MeterId','BillingMonth','MeterCategory','MeterSubcategory','Meter','AccountName','DepartmentName','SubscriptionId','SubscriptionName','ServiceName','ServiceTier','EnrollmentAccountName','BillingAccountId','ResourceGuid','BillingPeriod','InvoiceNumber','ChargeType','PublisherType','ReservationId','ReservationName','Frequency','PartNumber','CostAllocationRuleName','MarkupRuleName','PricingModel')  (space delimiter)" default:"ResourceType" default:"ResourceLocation"` //nolint:staticcheck
		}

		// portscan settings
		Portscan struct {
			Enabled   bool          `long:"portscan"                      env:"PORTSCAN"                                 description:"Enable portscan for public IPs"`
			Time      time.Duration `long:"portscan-time"                 env:"PORTSCAN_TIME"                            description:"Portscan time (time.duration)"                         default:"3h"`
			Parallel  int           `long:"portscan-parallel"             env:"PORTSCAN_PARALLEL"                        description:"Portscan parallel scans (parallel * threads = concurrent gofuncs)"  default:"2"`
			Threads   int           `long:"portscan-threads"              env:"PORTSCAN_THREADS"                         description:"Portscan threads (concurrent port scans per IP)"  default:"1000"`
			Timeout   int           `long:"portscan-timeout"              env:"PORTSCAN_TIMEOUT"                         description:"Portscan timeout (seconds)"                       default:"5"`
			PortRange []string      `long:"portscan-range"                env:"PORTSCAN_RANGE"            env-delim:" "  description:"Portscan port range (first-last)  (space delimiter)"                 default:"1-65535"`
		}

		// caching
		Cache struct {
			Path string `long:"cache-path"                    env:"CACHE_PATH"                               description:"Cache path"`
		}

		// general options
		ServerBind string `long:"bind"     env:"SERVER_BIND"   description:"Server address"     default:":8080"`
	}
)

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		log.Panic(err)
	}
	return jsonBytes
}
