package main

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-resourcemanager-exporter/config"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	Author                    = "webdevops.io"
	AZURE_RESOURCE_TAG_PREFIX = "tag_"
)

var (
	argparser *flags.Parser
	opts      config.Opts

	AzureAuthorizer    autorest.Authorizer
	AzureSubscriptions []subscriptions.Subscription

	azureResourceGroupTags AzureTagFilter
	azureResourceTags      AzureTagFilter
	azureEnvironment       azure.Environment
	portscanPortRange      []Portrange

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
			err := os.Mkdir(cacheDirectory, 0755)
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

	if opts.Scrape.TimeResource == nil {
		opts.Scrape.TimeResource = &opts.Scrape.Time
	}

	if opts.Scrape.TimeQuota == nil {
		opts.Scrape.TimeQuota = &opts.Scrape.Time
	}

	if opts.Scrape.TimeCompute == nil {
		opts.Scrape.TimeCompute = &opts.Scrape.Time
	}

	if opts.Scrape.TimeNetwork == nil {
		opts.Scrape.TimeNetwork = &opts.Scrape.Time
	}

	if opts.Scrape.TimeStorage == nil {
		opts.Scrape.TimeStorage = &opts.Scrape.Time
	}

	if opts.Scrape.TimeIam == nil {
		opts.Scrape.TimeIam = &opts.Scrape.Time
	}

	if opts.Scrape.TimeContainerRegistry == nil {
		opts.Scrape.TimeContainerRegistry = &opts.Scrape.Time
	}

	if opts.Scrape.TimeContainerInstance == nil {
		opts.Scrape.TimeContainerInstance = &opts.Scrape.Time
	}

	if opts.Scrape.TimeDatabase == nil {
		opts.Scrape.TimeDatabase = &opts.Scrape.Time
	}

	if opts.Scrape.TimeEventhub == nil {
		opts.Scrape.TimeEventhub = &opts.Scrape.Time
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

	azureResourceGroupTags = NewAzureTagFilter(AZURE_RESOURCE_TAG_PREFIX, opts.Azure.ResourceGroupTags)
	azureResourceTags = NewAzureTagFilter(AZURE_RESOURCE_TAG_PREFIX, opts.Azure.ResourceTags)

	// check deprecated env vars
	if os.Getenv("SCRAPE_TIME_COMPUTING") != "" {
		log.Panic("env var SCRAPE_TIME_COMPUTING is now SCRAPE_TIME_COMPUTE")
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
	subscriptionsClient := subscriptions.NewClient()
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

	azureEnvironment, err = azure.EnvironmentFromName(*opts.Azure.Environment)
	if err != nil {
		log.Panic(err)
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

	collectorName = "Compute"
	if opts.Scrape.TimeCompute.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmCompute{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeCompute)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Network"
	if opts.Scrape.TimeNetwork.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmNetwork{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeNetwork)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Storage"
	if opts.Scrape.TimeStorage.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmStorage{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeStorage)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "ContainerRegistry"
	if opts.Scrape.TimeContainerRegistry.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmContainerRegistry{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeContainerRegistry)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "ContainerInstance"
	if opts.Scrape.TimeContainerInstance.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmContainerInstances{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeContainerInstance)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "Database"
	if opts.Scrape.TimeDatabase.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmDatabase{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeDatabase)
	} else {
		log.WithField("collector", collectorName).Infof("collector disabled")
	}

	collectorName = "EventHub"
	if opts.Scrape.TimeDatabase.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorAzureRmEventhub{})
		collectorGeneralList[collectorName].Run(*opts.Scrape.TimeEventhub)
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
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}
