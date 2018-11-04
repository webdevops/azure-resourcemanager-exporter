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
	Version = "0.9.1"
	AZURE_RESOURCE_TAG_PREFIX = "tag_"
)

var (
	argparser          *flags.Parser
	args               []string
	Logger             *DaemonLogger
	ErrorLogger        *DaemonLogger
	AzureAuthorizer    autorest.Authorizer
	AzureSubscriptions []subscriptions.Subscription

	MetricCollector    *MetricCollectorAzureRm

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
	ServerBind  string `         long:"bind"                          env:"SERVER_BIND"                              description:"Server address"                                   default:":8080"`
	ScrapeTime  time.Duration `  long:"scrape-time"                   env:"SCRAPE_TIME"                              description:"Scrape time (time.duration)"                      default:"5m"`

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
	Logger.Messsage("  scape time: %v", opts.ScrapeTime)

	MetricCollector = &MetricCollectorAzureRm{}
	MetricCollector.Init(opts.Portscan)
	MetricCollector.Start()

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

// start and handle prometheus handler
func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	ErrorLogger.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}
