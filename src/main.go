package main

import (
	"os"
	"fmt"
	"time"
	"regexp"
	"errors"
	"context"
	"net/http"
	"path/filepath"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	Author  = "webdevops.io"
	Version = "0.10.1"
	AZURE_RESOURCE_TAG_PREFIX = "tag_"
)

var (
	argparser          *flags.Parser
	args               []string
	Logger             *DaemonLogger
	ErrorLogger        *DaemonLogger
	AzureAuthorizer    autorest.Authorizer
	AzureSubscriptions []subscriptions.Subscription

	collectorGeneralList    map[string]*MetricCollectorGeneral
	collectorCustomList     map[string]*MetricCollectorCustom

	portrangeRegexp = regexp.MustCompile("^(?P<first>[0-9]+)(-(?P<last>[0-9]+))?$")
)

type Portrange struct {
	FirstPort int
	LastPort int
}

var opts struct {
	// general settings
	Verbose     []bool `         long:"verbose" short:"v"             env:"VERBOSE"                                  description:"Verbose mode"`

	// server settings
	ServerBind  string `              long:"bind"                     env:"SERVER_BIND"                              description:"Server address"                                   default:":8080"`

	// scrape times
	ScrapeTime  time.Duration `                 long:"scrape-time"                    env:"SCRAPE_TIME"                    description:"Default scrape time (time.duration)"                      default:"5m"`
	ScrapeTimeGeneral  *time.Duration `         long:"scrape-time-general"            env:"SCRAPE_TIME_GENERAL"            description:"Scrape time for general metrics (time.duration)"`
	ScrapeTimeQuota *time.Duration `            long:"scrape-time-quota"              env:"SCRAPE_TIME_QUOTA"              description:"Scrape time for quota metrics  (time.duration)"`
	ScrapeTimeContainerRegistry *time.Duration `long:"scrape-time-containerregistry"  env:"SCRAPE_TIME_CONTAINERREGISTRY"  description:"Scrape time for ContainerRegistry metrics (time.duration)"`
	ScrapeTimeContainerInstance *time.Duration `long:"scrape-time-containerinstance"  env:"SCRAPE_TIME_CONTAINERINSTANCE"  description:"Scrape time for ContainerInstance metrics (time.duration)"`
	ScrapeTimeDatabase *time.Duration `         long:"scrape-time-database"           env:"SCRAPE_TIME_DATABASE"           description:"Scrape time for Database metrics (time.duration)"`
	ScrapeTimeSecurity *time.Duration `         long:"scrape-time-security"           env:"SCRAPE_TIME_SECURITY"           description:"Scrape time for Security metrics (time.duration)"`
	ScrapeTimeResourceHealth *time.Duration `   long:"scrape-time-resourcehealth"     env:"SCRAPE_TIME_RESOURCEHEALTH"     description:"Scrape time for ResourceHealth metrics (time.duration)"`
	ScrapeTimeComputing *time.Duration `        long:"scrape-time-computing"          env:"SCRAPE_TIME_COMPUTING"          description:"Scrape time for Computing metrics (time.duration)"`

	// azure settings
	AzureSubscription []string ` long:"azure-subscription"            env:"AZURE_SUBSCRIPTION_ID"     env-delim:" "  description:"Azure subscription ID"`
	AzureLocation []string `     long:"azure-location"                env:"AZURE_LOCATION"            env-delim:" "  description:"Azure locations"                                  default:"westeurope" default:"northeurope"`
	AzureResourceGroupTags []string `long:"azure-resourcegroup-tag"   env:"AZURE_RESOURCEGROUP_TAG"   env-delim:" "  description:"Azure ResourceGroup tags"                         default:"owner"`
	AzureResourceTags []string `long:"azure-resource-tag"             env:"AZURE_RESOURCE_TAG"        env-delim:" "  description:"Azure Resource tags"                              default:"owner"`

	// portscan settings
	Portscan  bool    `          long:"portscan"                      env:"PORTSCAN"                                 description:"Enable portscan for public IPs"`
	PortscanTime  time.Duration `long:"portscan-time"                 env:"PORTSCAN_TIME"                            description:"Portscan time (time.duration)"                         default:"3h"`
	PortscanPrallel  int    `    long:"portscan-parallel"             env:"PORTSCAN_PARALLEL"                        description:"Portscan parallel scans (parallel * threads = concurrent gofuncs)"  default:"2"`
	PortscanThreads  int    `    long:"portscan-threads"              env:"PORTSCAN_THREADS"                         description:"Portscan threads (concurrent port scans per IP)"  default:"1000"`
	PortscanTimeout  int    `    long:"portscan-timeout"              env:"PORTSCAN_TIMEOUT"                         description:"Portscan timeout (seconds)"                       default:"5"`
	PortscanPortRange []string  `long:"portscan-range"                env:"PORTSCAN_RANGE"            env-delim:" "  description:"Portscan port range (first-last)"                 default:"1-65535"`
	portscanPortRange []Portrange

	// caching
	CachePath string `           long:"cache-path"                    env:"CACHE_PATH"                               description:"Cache path"`
}

func main() {
	initArgparser()

	Logger = CreateDaemonLogger(0)
	ErrorLogger = CreateDaemonErrorLogger(0)

	// set verbosity
	Verbose = len(opts.Verbose) >= 1

	Logger.Messsage("Init Azure ResourceManager exporter v%s (written by %v)", Version, Author)

	Logger.Messsage("Init Azure connection")
	initAzureConnection()

	Logger.Messsage("Starting metrics collection")
	Logger.Messsage("  scape time General: %v", opts.ScrapeTimeGeneral)
	Logger.Messsage("  scape time Quota: %v", opts.ScrapeTimeQuota)
	Logger.Messsage("  scape time ContainerRegistry: %v", opts.ScrapeTimeContainerRegistry)
	Logger.Messsage("  scape time ContainerInstance: %v", opts.ScrapeTimeContainerInstance)
	Logger.Messsage("  scape time Database: %v", opts.ScrapeTimeDatabase)
	Logger.Messsage("  scape time Security: %v", opts.ScrapeTimeSecurity)
	Logger.Messsage("  scape time ResourceHealth: %v", opts.ScrapeTimeResourceHealth)

	if opts.Portscan {
		Logger.Messsage("  scape time Portscan: %v", opts.PortscanTime)
	}

	initMetricCollector()

	Logger.Messsage("Starting http server on %s", opts.ServerBind)
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
			fmt.Fprintf(os.Stderr, "%v%v\n", LoggerLogPrefixError, err.Error())
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

	if opts.ScrapeTimeQuota == nil {
		opts.ScrapeTimeQuota = &opts.ScrapeTime
	}

	if opts.ScrapeTimeComputing == nil {
		opts.ScrapeTimeComputing = &opts.ScrapeTime
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

	if opts.ScrapeTimeSecurity == nil {
		opts.ScrapeTimeSecurity = &opts.ScrapeTime
	}

	if opts.ScrapeTimeResourceHealth == nil {
		opts.ScrapeTimeResourceHealth = &opts.ScrapeTime
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
}

func initMetricCollector() {
	var collectorName string
	collectorGeneralList = map[string]*MetricCollectorGeneral{}
	collectorCustomList = map[string]*MetricCollectorCustom{}

	collectorName = "General"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorGeneralList[collectorName] = &MetricCollectorGeneral{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorAzureRmGeneral{}}
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeGeneral)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Quota"
	if opts.ScrapeTimeQuota.Seconds() > 0 {
		collectorGeneralList[collectorName] = &MetricCollectorGeneral{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorAzureRmQuota{}}
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeQuota)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Computing"
	if opts.ScrapeTimeContainerRegistry.Seconds() > 0 {
		collectorGeneralList[collectorName] = &MetricCollectorGeneral{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorAzureRmComputing{}}
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeContainerRegistry)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "ContainerRegistry"
	if opts.ScrapeTimeContainerRegistry.Seconds() > 0 {
		collectorGeneralList[collectorName] = &MetricCollectorGeneral{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorAzureRmContainerRegistry{}}
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeContainerRegistry)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "ContainerInstance"
	if opts.ScrapeTimeContainerInstance.Seconds() > 0 {
		collectorGeneralList[collectorName] = &MetricCollectorGeneral{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorAzureRmContainerInstances{}}
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeContainerInstance)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Database"
	if opts.ScrapeTimeDatabase.Seconds() > 0 {
		collectorGeneralList[collectorName] = &MetricCollectorGeneral{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorAzureRmDatabase{}}
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeDatabase)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Security"
	if opts.ScrapeTimeSecurity.Seconds() > 0 {
		collectorGeneralList[collectorName] = &MetricCollectorGeneral{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorAzureRmSecurity{}}
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeSecurity)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Health"
	if opts.ScrapeTimeResourceHealth.Seconds() > 0 {
		collectorGeneralList[collectorName] = &MetricCollectorGeneral{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorAzureRmHealth{}}
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeResourceHealth)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Portscan"
	if opts.Portscan {
		collectorCustomList[collectorName] = &MetricCollectorCustom{Name:collectorName, AzureSubscriptions:AzureSubscriptions,Collector:&MetricsCollectorPortscanner{}}
		collectorCustomList[collectorName].Run(opts.PortscanTime)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}
}

// start and handle prometheus handler
func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	ErrorLogger.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}
