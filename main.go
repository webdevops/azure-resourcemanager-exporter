package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	flags "github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/prometheus/azuretracing"

	"github.com/webdevops/azure-resourcemanager-exporter/config"
)

const (
	Author    = "webdevops.io"
	UserAgent = "azure-metrics-exporter/"
)

var (
	argparser *flags.Parser
	opts      config.Opts

	AzureAuthorizer    autorest.Authorizer
	AzureSubscriptions []subscriptions.Subscription

	azureEnvironment  azure.Environment
	portscanPortRange []Portrange

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

func main() {
	initArgparser()

	log.Infof("starting azure-resourcemanager-exporter v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	log.Info(string(opts.GetJson()))

	log.Infof("init Azure connection")
	initAzureConnection()

	log.Infof("starting metrics collection")
	initMetricCollector()

	log.Infof("starting http server on %s", opts.ServerBind)
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
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	// verbose level
	if opts.Logger.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	// debug level
	if opts.Logger.Debug {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
		log.SetFormatter(&log.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}

	// json log format
	if opts.Logger.LogJson {
		log.SetReportCaller(true)
		log.SetFormatter(&log.JSONFormatter{
			DisableTimestamp: true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}

	if opts.Portscan.Enabled {
		// parse --portscan-range
		err := argparserParsePortrange()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v%v\n", "[ERROR] ", err.Error())
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	if opts.Cache.Path != "" {
		cacheDirectory := filepath.Dir(opts.Cache.Path)
		if _, err := os.Stat(cacheDirectory); os.IsNotExist(err) {
			err := os.Mkdir(cacheDirectory, 0700)
			if err != nil {
				log.Panic(err)
			}
		}
	}

	// deprecated option
	if len(opts.Azure.ResourceGroupTags) > 0 {
		opts.Azure.ResourceTags = opts.Azure.ResourceGroupTags
	}

	// scrape time
	if opts.Scrape.TimeGeneral == nil {
		opts.Scrape.TimeGeneral = &opts.Scrape.Time
	}

	if opts.Scrape.TimeRateLimitRead == nil {
		opts.Scrape.TimeRateLimitRead = &opts.Scrape.Time
	}

	if opts.Scrape.TimeRateLimitWrite == nil {
		opts.Scrape.TimeRateLimitWrite = &opts.Scrape.Time
	}

	if opts.Scrape.TimeResource == nil {
		opts.Scrape.TimeResource = &opts.Scrape.Time
	}

	if opts.Scrape.TimeQuota == nil {
		opts.Scrape.TimeQuota = &opts.Scrape.Time
	}

	if opts.Scrape.TimeCosts == nil {
		opts.Scrape.TimeCosts = &opts.Scrape.Time
	}

	if opts.Scrape.TimeIam == nil {
		opts.Scrape.TimeIam = &opts.Scrape.Time
	}

	if opts.Scrape.TimeSecurity == nil {
		opts.Scrape.TimeSecurity = &opts.Scrape.Time
	}

	if opts.Scrape.TimeResourceHealth == nil {
		opts.Scrape.TimeResourceHealth = &opts.Scrape.Time
	}

	if opts.Scrape.TimeGraph == nil {
		opts.Scrape.TimeGraph = &opts.Scrape.Time
	}

	// check deprecated env vars
	deprecatedEnvVars := map[string]string{
		"SCRAPE_TIME_CONTAINERREGISTRY": "not supported anymore",
		"SCRAPE_TIME_CONTAINERINSTANCE": "not supported anymore",
		"SCRAPE_TIME_EVENTHUB":          "not supported anymore",
		"SCRAPE_TIME_STORAGE":           "not supported anymore",
		"SCRAPE_TIME_COMPUTE":           "not supported anymore",
		"SCRAPE_TIME_NETWORK":           "not supported anymore",
		"SCRAPE_TIME_DATABASE":          "not supported anymore",
		"SCRAPE_TIME_COMPUTING":         "deprecated, please use SCRAPE_TIME_COMPUTE",
	}
	for envVar, reason := range deprecatedEnvVars {
		if os.Getenv(envVar) != "" {
			log.Panicf("env var %v is %v", envVar, reason)
		}
	}
}

// Init and build Azure authorzier
func initAzureConnection() {
	var err error
	ctx := context.Background()

	// setup azure authorizer
	AzureAuthorizer, err = auth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Panic(err)
	}

	azureEnvironment, err = azure.EnvironmentFromName(*opts.Azure.Environment)
	if err != nil {
		log.Panic(err)
	}

	subscriptionsClient := subscriptions.NewClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint)
	subscriptionsClient.Authorizer = AzureAuthorizer

	if len(opts.Azure.Subscription) == 0 {
		// auto lookup subscriptions
		listResult, err := subscriptionsClient.List(ctx)
		if err != nil {
			log.Panic(err)
		}
		AzureSubscriptions = listResult.Values()

		if len(AzureSubscriptions) == 0 {
			log.Panic("no Azure Subscriptions found via auto detection, does this ServicePrincipal have read permissions to the subcriptions?")
		}
	} else {
		// fixed subscription list
		AzureSubscriptions = []subscriptions.Subscription{}
		for _, subId := range opts.Azure.Subscription {
			result, err := subscriptionsClient.Get(ctx, subId)
			if err != nil {
				log.Panic(err)
			}
			AzureSubscriptions = append(AzureSubscriptions, result)
		}
	}
}

func initMetricCollector() {
	var collectorName string
	collectorGeneralList = map[string]*CollectorGeneral{}
	collectorCustomList = map[string]*CollectorCustom{}

	collectorName = "General"
	if opts.Scrape.TimeGeneral.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmGeneral{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeGeneral)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "RateLimitRead"
	if opts.Scrape.TimeRateLimitRead.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmRateLimitRead{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeRateLimitRead)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}
	collectorName = "RateLimitWrite"
	if opts.Scrape.TimeRateLimitWrite.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmRateLimitWrite{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeRateLimitWrite)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Resource"
	if opts.Scrape.TimeResource.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmResources{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeResource)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Quota"
	if opts.Scrape.TimeQuota.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmQuota{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeQuota)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Costs"
	if opts.Scrape.TimeCosts.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmCosts{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeCosts)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Security"
	if opts.Scrape.TimeSecurity.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmSecurity{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeSecurity)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Health"
	if opts.Scrape.TimeResourceHealth.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmHealth{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeResourceHealth)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "IAM"
	if opts.Scrape.TimeIam.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmIam{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeIam)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "GraphApps"
	if opts.Scrape.TimeGraph.Seconds() > 0 {
		collectorCustomList[collectorName] = NewCollectorCustom(collectorName, &MetricsCollectorGraphApps{})
		collectorCustomList[collectorName].Run(*opts.Scrape.TimeGraph)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Portscan"
	if opts.Portscan.Enabled {
		collectorCustomList[collectorName] = NewCollectorCustom(collectorName, &MetricsCollectorPortscanner{})
		collectorCustomList[collectorName].Run(opts.Portscan.Time)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Exporter"
	if opts.Scrape.TimeExporter.Seconds() > 0 {
		collectorCustomList[collectorName] = NewCollectorCustom(collectorName, &MetricsCollectorExporter{})
		collectorCustomList[collectorName].SetIsHidden(true)
		collectorCustomList[collectorName].Run(*opts.Scrape.TimeExporter)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

}

// start and handle prometheus handler
func startHttpServer() {
	// healthz
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			log.Error(err)
		}
	})

	http.Handle("/metrics", azuretracing.RegisterAzureMetricAutoClean(promhttp.Handler()))

	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

func decorateAzureAutorest(client *autorest.Client) {
	client.Authorizer = AzureAuthorizer
	if err := client.AddToUserAgent(UserAgent + gitTag); err != nil {
		log.Panic(err)
	}

	azuretracing.DecorateAzureAutoRestClient(client)
}
