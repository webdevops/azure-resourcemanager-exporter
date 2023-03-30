package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"time"

	flags "github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/azuresdk/azidentity"
	"github.com/webdevops/go-common/azuresdk/prometheus/tracing"
	"github.com/webdevops/go-common/msgraphsdk/msgraphclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"go.uber.org/zap"

	"github.com/webdevops/azure-resourcemanager-exporter/config"
)

const (
	Author    = "webdevops.io"
	UserAgent = "azure-resourcemanager-exporter/"
)

var (
	argparser *flags.Parser
	opts      config.Opts

	AzureClient                *armclient.ArmClient
	AzureSubscriptionsIterator *armclient.SubscriptionsIterator

	MsGraphClient *msgraphclient.MsGraphClient

	portscanPortRange []Portrange

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
	defer initLogger().Sync() // nolint:errcheck

	logger.Infof("starting azure-resourcemanager-exporter v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	logger.Info(string(opts.GetJson()))
	logger.Warnf("test")

	logger.Infof("init Azure connection")
	initAzureConnection()

	logger.Infof("starting metrics collection")
	initMetricCollector()

	logger.Infof("starting http server on %s", opts.Server.Bind)
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		var flagsErr *flags.Error
		if ok := errors.As(err, &flagsErr); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
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

	// scrape time
	if opts.Scrape.Time.General == nil {
		opts.Scrape.Time.General = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.Resource == nil {
		opts.Scrape.Time.Resource = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.Quota == nil {
		opts.Scrape.Time.Quota = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.Costs == nil {
		opts.Scrape.Time.Costs = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.Iam == nil {
		opts.Scrape.Time.Iam = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.Defender == nil {
		opts.Scrape.Time.Defender = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.ResourceHealth == nil {
		opts.Scrape.Time.ResourceHealth = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.Graph == nil {
		opts.Scrape.Time.Graph = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.Portscan == nil {
		opts.Scrape.Time.Portscan = &opts.Scrape.Time.Default
	}

	if opts.Scrape.Time.Portscan == nil || opts.Scrape.Time.Portscan.Seconds() == 0 && opts.Portscan.Enabled {
		logger.Fatalf(`portscan is enabled but has invalid scape time (zero)`)
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
		"SCRAPE_TIME_SECURITY":          "deprecated, please use SCRAPE_TIME_DEFENDER",
	}
	for envVar, reason := range deprecatedEnvVars {
		if os.Getenv(envVar) != "" {
			logger.Fatalf("env var %v is %v", envVar, reason)
		}
	}
}

func initAzureConnection() {
	var err error

	if opts.Azure.Environment != nil {
		if err := os.Setenv(azidentity.EnvAzureEnvironment, *opts.Azure.Environment); err != nil {
			logger.Warnf(`unable to set envvar "%s": %v`, azidentity.EnvAzureEnvironment, err.Error())
		}
	}

	AzureClient, err = armclient.NewArmClientFromEnvironment(logger)
	if err != nil {
		logger.Fatal(err.Error())
	}
	AzureClient.SetUserAgent(UserAgent + gitTag)

	// limit subscriptions (if filter is set)
	if len(opts.Azure.Subscription) >= 1 {
		AzureClient.SetSubscriptionFilter(opts.Azure.Subscription...)
	}

	if err := AzureClient.Connect(); err != nil {
		logger.Fatal(err.Error())
	}

	AzureSubscriptionsIterator = armclient.NewSubscriptionIterator(AzureClient)
}

func initMsGraphConnection() {
	var err error
	if MsGraphClient == nil {
		MsGraphClient, err = msgraphclient.NewMsGraphClientWithCloudName(*opts.Azure.Environment, *opts.Azure.Tenant, logger)
		if err != nil {
			logger.Fatal(err.Error())
		}

		MsGraphClient.SetUserAgent(UserAgent + gitTag)
	}
}

func initMetricCollector() {
	var collectorName string

	collectorName = "General"
	if opts.Scrape.Time.General.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorAzureRmGeneral{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.General)
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "Resource"
	if opts.Scrape.Time.Resource.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorAzureRmResources{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.Resource)
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "Quota"
	if opts.Scrape.Time.Quota.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorAzureRmQuota{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.Quota)
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "Costs"
	if opts.Scrape.Time.Costs.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorAzureRmCosts{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.Costs)
		// higher backoff times because of strict cost rate limits
		c.SetBackoffDurations(
			2*time.Minute,
			5*time.Minute,
			10*time.Minute,
		)
		c.SetCache(opts.GetCachePath("costs.json"))
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "Defender"
	if opts.Scrape.Time.Defender.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorAzureRmDefender{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.Defender)
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "ResourceHealth"
	if opts.Scrape.Time.ResourceHealth.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorAzureRmHealth{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.ResourceHealth)
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "IAM"
	if opts.Scrape.Time.Iam.Seconds() > 0 {
		initMsGraphConnection()
		c := collector.New(collectorName, &MetricsCollectorAzureRmIam{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.Iam)
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "GraphApplications"
	if opts.Scrape.Time.Graph.Seconds() > 0 {
		initMsGraphConnection()
		c := collector.New(collectorName, &MetricsCollectorGraphApps{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.Graph)
		c.SetCache(opts.GetCachePath("graphApplications.json"))
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "GraphServicePrincipals"
	if opts.Scrape.Time.Graph.Seconds() > 0 {
		initMsGraphConnection()
		c := collector.New(collectorName, &MetricsCollectorGraphServicePrincipals{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.Graph)
		c.SetCache(opts.GetCachePath("graphServicePrincipals.json"))
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "Portscan"
	if opts.Portscan.Enabled && opts.Scrape.Time.Portscan.Seconds() > 0 {
		c := collector.New(collectorName, &MetricsCollectorPortscanner{}, logger)
		c.SetScapeTime(*opts.Scrape.Time.Portscan)
		c.SetCache(opts.GetCachePath("portscanner.json"))
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(zap.String("collector", collectorName)).Infof("collector disabled")
	}
}

// start and handle prometheus handler
func startHttpServer() {
	mux := http.NewServeMux()

	// healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	// readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	mux.Handle("/metrics", collector.HttpWaitForRlock(
		tracing.RegisterAzureMetricAutoClean(promhttp.Handler())),
	)

	srv := &http.Server{
		Addr:         opts.Server.Bind,
		Handler:      mux,
		ReadTimeout:  opts.Server.ReadTimeout,
		WriteTimeout: opts.Server.WriteTimeout,
	}
	logger.Fatal(srv.ListenAndServe())
}
