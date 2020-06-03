package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	Author                    = "webdevops.io"
	AZURE_RESOURCE_TAG_PREFIX = "tag_"
)

var (
	argparser          *flags.Parser
	Verbose            bool
	Logger             *DaemonLogger
	AzureAuthorizer    autorest.Authorizer
	AzureSubscriptions []subscriptions.Subscription

	collectorGeneralList map[string]*CollectorGeneral
	collectorCustomList  map[string]*CollectorCustom

	portrangeRegexp = regexp.MustCompile("^(?P<first>[0-9]+)(-(?P<last>[0-9]+))?$")

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

type Portrange struct {
	FirstPort int
	LastPort  int
}

var opts struct {
	// general settings
	Verbose []bool `long:"verbose" short:"v"             env:"VERBOSE"                                  description:"Verbose mode"`

	// server settings
	ServerBind string `long:"bind"                     env:"SERVER_BIND"                              description:"Server address"                                   default:":8080"`

	// scrape times
	ScrapeTime                  time.Duration  `long:"scrape-time"                    env:"SCRAPE_TIME"                    description:"Default scrape time (time.duration)"                      default:"5m"`
	ScrapeTimeExporter          *time.Duration `long:"scrape-time-exporter"           env:"SCRAPE_TIME_EXPORTER"           description:"Scrape time for exporter metrics (time.duration)"         default:"10s"`
	ScrapeTimeGeneral           *time.Duration `long:"scrape-time-general"            env:"SCRAPE_TIME_GENERAL"            description:"Scrape time for general metrics (time.duration)"`
	ScrapeTimeResource          *time.Duration `long:"scrape-time-resource"           env:"SCRAPE_TIME_RESOURCE"           description:"Scrape time for resource metrics  (time.duration)"`
	ScrapeTimeQuota             *time.Duration `long:"scrape-time-quota"              env:"SCRAPE_TIME_QUOTA"              description:"Scrape time for quota metrics  (time.duration)"`
	ScrapeTimeContainerRegistry *time.Duration `long:"scrape-time-containerregistry"  env:"SCRAPE_TIME_CONTAINERREGISTRY"  description:"Scrape time for ContainerRegistry metrics (time.duration)"`
	ScrapeTimeContainerInstance *time.Duration `long:"scrape-time-containerinstance"  env:"SCRAPE_TIME_CONTAINERINSTANCE"  description:"Scrape time for ContainerInstance metrics (time.duration)"`
	ScrapeTimeDatabase          *time.Duration `long:"scrape-time-database"           env:"SCRAPE_TIME_DATABASE"           description:"Scrape time for Database metrics (time.duration)"`
	ScrapeTimeSecurity          *time.Duration `long:"scrape-time-security"           env:"SCRAPE_TIME_SECURITY"           description:"Scrape time for Security metrics (time.duration)"`
	ScrapeTimeResourceHealth    *time.Duration `long:"scrape-time-resourcehealth"     env:"SCRAPE_TIME_RESOURCEHEALTH"     description:"Scrape time for ResourceHealth metrics (time.duration)"`
	ScrapeTimeCompute           *time.Duration `long:"scrape-time-compute"            env:"SCRAPE_TIME_COMPUTE"            description:"Scrape time for Compute metrics (time.duration)"`
	ScrapeTimeNetwork           *time.Duration `long:"scrape-time-network"            env:"SCRAPE_TIME_NETWORK"            description:"Scrape time for Network metrics (time.duration)"`
	ScrapeTimeEventhub          *time.Duration `long:"scrape-time-eventhub"           env:"SCRAPE_TIME_EVENTHUB"           description:"Scrape time for Eventhub metrics (time.duration)"         default:"30m"`
	ScrapeTimeStorage           *time.Duration `long:"scrape-time-storage"            env:"SCRAPE_TIME_STORAGE"            description:"Scrape time for Storage metrics (time.duration)"`
	ScrapeTimeIam               *time.Duration `long:"scrape-time-iam"                env:"SCRAPE_TIME_IAM"                description:"Scrape time for IAM metrics (time.duration)"`
	ScrapeTimeGraph             *time.Duration `long:"scrape-time-graph"              env:"SCRAPE_TIME_GRAPH"              description:"Scrape time for Graph metrics (time.duration)"`

	// azure settings
	AzureSubscription      []string `long:"azure-subscription"            env:"AZURE_SUBSCRIPTION_ID"     env-delim:" "  description:"Azure subscription ID"`
	AzureLocation          []string `long:"azure-location"                env:"AZURE_LOCATION"            env-delim:" "  description:"Azure locations"                                  default:"westeurope" default:"northeurope"`
	AzureResourceGroupTags []string `long:"azure-resourcegroup-tag"   env:"AZURE_RESOURCEGROUP_TAG"   env-delim:" "  description:"Azure ResourceGroup tags"                         default:"owner"`
	azureResourceGroupTags AzureTagFilter
	AzureResourceTags      []string `long:"azure-resource-tag"             env:"AZURE_RESOURCE_TAG"        env-delim:" "  description:"Azure Resource tags"                              default:"owner"`
	azureResourceTags      AzureTagFilter
	azureEnvironment       azure.Environment

	// graph settings
	GraphApplicationFilter string `long:"graph-application-filter"    env:"GRAPH_APPLICATION_FILTER"               description:"Graph application filter query eg: startswith(displayName,'A')"`

	// portscan settings
	Portscan          bool          `long:"portscan"                      env:"PORTSCAN"                                 description:"Enable portscan for public IPs"`
	PortscanTime      time.Duration `long:"portscan-time"                 env:"PORTSCAN_TIME"                            description:"Portscan time (time.duration)"                         default:"3h"`
	PortscanPrallel   int           `long:"portscan-parallel"             env:"PORTSCAN_PARALLEL"                        description:"Portscan parallel scans (parallel * threads = concurrent gofuncs)"  default:"2"`
	PortscanThreads   int           `long:"portscan-threads"              env:"PORTSCAN_THREADS"                         description:"Portscan threads (concurrent port scans per IP)"  default:"1000"`
	PortscanTimeout   int           `long:"portscan-timeout"              env:"PORTSCAN_TIMEOUT"                         description:"Portscan timeout (seconds)"                       default:"5"`
	PortscanPortRange []string      `long:"portscan-range"                env:"PORTSCAN_RANGE"            env-delim:" "  description:"Portscan port range (first-last)"                 default:"1-65535"`
	portscanPortRange []Portrange

	// caching
	CachePath string `long:"cache-path"                    env:"CACHE_PATH"                               description:"Cache path"`
}

func main() {
	initArgparser()

	// set verbosity
	Verbose = len(opts.Verbose) >= 1

	Logger = NewLogger(log.Lshortfile, Verbose)
	defer Logger.Close()

	// set verbosity
	Verbose = len(opts.Verbose) >= 1

	Logger.Infof("Init Azure ResourceManager exporter v%s (%s; by %v)", gitTag, gitCommit, Author)

	Logger.Infof("Init Azure connection")
	initAzureConnection()

	Logger.Infof("Starting metrics collection")
	Logger.Infof("  scape time General: %v", opts.ScrapeTimeGeneral)
	Logger.Infof("  scape time Quota: %v", opts.ScrapeTimeQuota)
	Logger.Infof("  scape time ContainerRegistry: %v", opts.ScrapeTimeContainerRegistry)
	Logger.Infof("  scape time ContainerInstance: %v", opts.ScrapeTimeContainerInstance)
	Logger.Infof("  scape time Database: %v", opts.ScrapeTimeDatabase)
	Logger.Infof("  scape time Security: %v", opts.ScrapeTimeSecurity)
	Logger.Infof("  scape time ResourceHealth: %v", opts.ScrapeTimeResourceHealth)
	Logger.Infof("  scape time Graph: %v", opts.ScrapeTimeGraph)

	if opts.Portscan {
		Logger.Infof("  scape time Portscan: %v", opts.PortscanTime)
	}

	initMetricCollector()

	Logger.Infof("Starting http server on %s", opts.ServerBind)
	startHttpServer()
}

// init argparser and parse/validate arguments
func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println(err)
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	if opts.Portscan {

		// parse --portscan-range
		err := argparserParsePortrange()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v%v\n", "[ERROR] ", err.Error())
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	if opts.CachePath != "" {
		cacheDirectory := filepath.Dir(opts.CachePath)
		if _, err := os.Stat(cacheDirectory); os.IsNotExist(err) {
			err := os.Mkdir(cacheDirectory, 0755)
			if err != nil {
				panic(err)
			}
		}
	}

	// deprecated option
	if len(opts.AzureResourceGroupTags) > 0 {
		opts.AzureResourceTags = opts.AzureResourceGroupTags
	}

	// scrape time
	if opts.ScrapeTimeGeneral == nil {
		opts.ScrapeTimeGeneral = &opts.ScrapeTime
	}

	if opts.ScrapeTimeResource == nil {
		opts.ScrapeTimeResource = &opts.ScrapeTime
	}

	if opts.ScrapeTimeQuota == nil {
		opts.ScrapeTimeQuota = &opts.ScrapeTime
	}

	if opts.ScrapeTimeCompute == nil {
		opts.ScrapeTimeCompute = &opts.ScrapeTime
	}

	if opts.ScrapeTimeNetwork == nil {
		opts.ScrapeTimeNetwork = &opts.ScrapeTime
	}

	if opts.ScrapeTimeStorage == nil {
		opts.ScrapeTimeStorage = &opts.ScrapeTime
	}

	if opts.ScrapeTimeIam == nil {
		opts.ScrapeTimeIam = &opts.ScrapeTime
	}

	if opts.ScrapeTimeContainerRegistry == nil {
		opts.ScrapeTimeContainerRegistry = &opts.ScrapeTime
	}

	if opts.ScrapeTimeContainerInstance == nil {
		opts.ScrapeTimeContainerInstance = &opts.ScrapeTime
	}

	if opts.ScrapeTimeDatabase == nil {
		opts.ScrapeTimeDatabase = &opts.ScrapeTime
	}

	if opts.ScrapeTimeEventhub == nil {
		opts.ScrapeTimeEventhub = &opts.ScrapeTime
	}

	if opts.ScrapeTimeSecurity == nil {
		opts.ScrapeTimeSecurity = &opts.ScrapeTime
	}

	if opts.ScrapeTimeResourceHealth == nil {
		opts.ScrapeTimeResourceHealth = &opts.ScrapeTime
	}

	if opts.ScrapeTimeGraph == nil {
		opts.ScrapeTimeGraph = &opts.ScrapeTime
	}

	opts.azureResourceGroupTags = NewAzureTagFilter(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceGroupTags)
	opts.azureResourceTags = NewAzureTagFilter(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)

	// check deprecated env vars
	if os.Getenv("SCRAPE_TIME_COMPUTING") != "" {
		panic("env var SCRAPE_TIME_COMPUTING is now SCRAPE_TIME_COMPUTE")
	}
}

// Init and build Azure authorzier
func initAzureConnection() {
	var err error
	ctx := context.Background()

	// setup azure authorizer
	AzureAuthorizer, err = auth.NewAuthorizerFromEnvironment()
	if err != nil {
		panic(err)
	}
	subscriptionsClient := subscriptions.NewClient()
	subscriptionsClient.Authorizer = AzureAuthorizer

	if len(opts.AzureSubscription) == 0 {
		// auto lookup subscriptions
		listResult, err := subscriptionsClient.List(ctx)
		if err != nil {
			panic(err)
		}
		AzureSubscriptions = listResult.Values()

		if len(AzureSubscriptions) == 0 {
			panic(errors.New("No Azure Subscriptions found via auto detection, does this ServicePrincipal have read permissions to the subcriptions?"))
		}
	} else {
		// fixed subscription list
		AzureSubscriptions = []subscriptions.Subscription{}
		for _, subId := range opts.AzureSubscription {
			result, err := subscriptionsClient.Get(ctx, subId)
			if err != nil {
				panic(err)
			}
			AzureSubscriptions = append(AzureSubscriptions, result)
		}
	}

	// try to get cloud name, defaults to public cloud name
	azureEnvName := azure.PublicCloud.Name
	if env := os.Getenv("AZURE_ENVIRONMENT"); env != "" {
		azureEnvName = env
	}

	opts.azureEnvironment, err = azure.EnvironmentFromName(azureEnvName)
	if err != nil {
		panic(err)
	}
}

func initMetricCollector() {
	var collectorName string
	collectorGeneralList = map[string]*CollectorGeneral{}
	collectorCustomList = map[string]*CollectorCustom{}

	collectorName = "General"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmGeneral{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeGeneral)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Resource"
	if opts.ScrapeTimeResource.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmResources{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeResource)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Quota"
	if opts.ScrapeTimeQuota.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmQuota{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeQuota)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Compute"
	if opts.ScrapeTimeCompute.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmCompute{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeCompute)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Network"
	if opts.ScrapeTimeNetwork.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmNetwork{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeNetwork)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Storage"
	if opts.ScrapeTimeStorage.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmStorage{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeStorage)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "ContainerRegistry"
	if opts.ScrapeTimeContainerRegistry.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmContainerRegistry{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeContainerRegistry)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "ContainerInstance"
	if opts.ScrapeTimeContainerInstance.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmContainerInstances{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeContainerInstance)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Database"
	if opts.ScrapeTimeDatabase.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmDatabase{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeDatabase)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "EventHub"
	if opts.ScrapeTimeDatabase.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmEventhub{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeEventhub)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Security"
	if opts.ScrapeTimeSecurity.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmSecurity{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeSecurity)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Health"
	if opts.ScrapeTimeResourceHealth.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmHealth{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeResourceHealth)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "IAM"
	if opts.ScrapeTimeIam.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmIam{})
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeIam)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "GraphApps"
	if opts.ScrapeTimeGraph.Seconds() > 0 {
		collectorCustomList[collectorName] = NewCollectorCustom(collectorName, &MetricsCollectorGraphApps{})
		collectorCustomList[collectorName].Run(*opts.ScrapeTimeGraph)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Portscan"
	if opts.Portscan {
		collectorCustomList[collectorName] = NewCollectorCustom(collectorName, &MetricsCollectorPortscanner{})
		collectorCustomList[collectorName].Run(opts.PortscanTime)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

	collectorName = "Exporter"
	if opts.ScrapeTimeExporter.Seconds() > 0 {
		collectorCustomList[collectorName] = NewCollectorCustom(collectorName, &MetricsCollectorExporter{})
		collectorCustomList[collectorName].SetIsHidden(true)
		collectorCustomList[collectorName].Run(*opts.ScrapeTimeExporter)
	} else {
		Logger.Infof("collector[%s]: disabled", collectorName)
	}

}

// start and handle prometheus handler
func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	Logger.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}
