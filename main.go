package main

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"time"

	yaml "github.com/goccy/go-yaml"

	"github.com/webdevops/azure-resourcemanager-exporter/config"

	flags "github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/azuresdk/azidentity"
	"github.com/webdevops/go-common/azuresdk/prometheus/tracing"
	"github.com/webdevops/go-common/msgraphsdk/msgraphclient"
	"github.com/webdevops/go-common/prometheus/collector"
)

const (
	Author    = "webdevops.io"
	UserAgent = "az-rm-exporter/"
)

var (
	argparser *flags.Parser
	Opts      config.Opts
	Config    config.Config

	//go:embed default.yaml
	defaultConfig []byte

	AzureClient                  *armclient.ArmClient
	AzureSubscriptionsIterator   *armclient.SubscriptionsIterator
	AzureResourceTagManager      *armclient.ResourceTagManager
	AzureResourceGroupTagManager *armclient.ResourceTagManager

	MsGraphClient *msgraphclient.MsGraphClient

	portscanPortRange []Portrange

	portrangeRegexp = regexp.MustCompile("^(?P<first>[0-9]+)(-(?P<last>[0-9]+))?$")

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
	buildDate = "<unknown>"

	// cache config
	cacheTag = "v1"
)

type Portrange struct {
	FirstPort int
	LastPort  int
}

func main() {
	initArgparser()
	initLogger()
	initConfig()

	logger.Infof("starting azure-resourcemanager-exporter v%s (%s; %s; by %v at %v)", gitTag, gitCommit, runtime.Version(), Author, buildDate)
	logger.Info(string(Opts.GetJson()))
	logger.Info(string(Config.GetJson()))
	initSystem()

	logger.Infof("init Azure connection")
	initAzureConnection()

	logger.Infof("starting metrics collection")
	initMetricCollector()

	logger.Info("starting http server", slog.String("bind", Opts.Server.Bind))
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&Opts, flags.Default)
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
}

func initConfig() {
	var err error

	err = yaml.UnmarshalWithOptions(defaultConfig, &Config, yaml.Strict(), yaml.UseJSONUnmarshaler())
	if err != nil {
		logger.Fatal(err.Error())
	}

	logger.Infof(`reading config from "%v"`, Opts.Config)
	/* #nosec */
	content, err := os.ReadFile(Opts.Config)
	if err != nil {
		logger.Fatal(err.Error())
	}

	err = yaml.UnmarshalWithOptions(content, &Config, yaml.Strict(), yaml.UseJSONUnmarshaler())
	if err != nil {
		logger.Fatal(err.Error())
	}
}

func initAzureConnection() {
	var err error

	if Opts.Azure.Environment != nil {
		if err := os.Setenv(azidentity.EnvAzureEnvironment, *Opts.Azure.Environment); err != nil {
			logger.Warn(`unable to set environment variable`, slog.String("env", azidentity.EnvAzureEnvironment), slog.Any("error", err))
		}
	}

	AzureClient, err = armclient.NewArmClientFromEnvironment(logger.Slog())
	if err != nil {
		logger.Fatal(err.Error())
	}
	AzureClient.SetUserAgent(UserAgent + gitTag)

	// limit subscriptions (if filter is set)
	if len(Config.Azure.Subscriptions) >= 1 {
		AzureClient.AddSubscriptionID(Config.Azure.Subscriptions...)
	}

	if err := AzureClient.Connect(); err != nil {
		logger.Fatal(err.Error())
	}

	// init subscription iterator
	AzureSubscriptionsIterator = armclient.NewSubscriptionIterator(AzureClient, Config.Azure.Subscriptions...)

	// init resource tag manager
	AzureResourceTagManager, err = AzureClient.TagManager.ParseTagConfig(Config.Azure.ResourceTags)
	if err != nil {
		logger.Fatal(`unable to parse resourceTag configuration`, slog.Any("config", Config.Azure.ResourceTags), slog.Any("error", err.Error()))
	}

	// init resourceGroup tag manager
	AzureResourceGroupTagManager, err = AzureClient.TagManager.ParseTagConfig(Config.Azure.ResourceGroupTags)
	if err != nil {
		logger.Fatal(`unable to parse resourceGroupTag configuration`, slog.Any("config", Config.Azure.ResourceGroupTags), slog.Any("error", err.Error()))
	}
}

func initMsGraphConnection() {
	var err error
	if MsGraphClient == nil {
		MsGraphClient, err = msgraphclient.NewMsGraphClientWithCloudName(*Opts.Azure.Environment, *Opts.Azure.Tenant, logger.Slog())
		if err != nil {
			logger.Fatal(err.Error())
		}

		MsGraphClient.SetUserAgent(UserAgent + gitTag)
	}
}

type CollectorDefinition struct {
	Name      string
	Config    config.CollectorBaseInterface
	Processor collector.ProcessorInterface
	Init      func()
	Setup     func(*collector.Collector)
}

func initMetricCollector() {
	collectorList := []CollectorDefinition{
		{
			Name:      "general",
			Config:    Config.Collectors.General,
			Processor: &MetricsCollectorAzureRmGeneral{},
		},
		{
			Name:      "resource",
			Config:    Config.Collectors.Resource,
			Processor: &MetricsCollectorAzureRmResources{},
		},
		{
			Name:      "quota",
			Config:    Config.Collectors.Quota,
			Processor: &MetricsCollectorAzureRmQuota{},
		},
		{
			Name:      "costs",
			Config:    Config.Collectors.Costs,
			Processor: &MetricsCollectorAzureRmCosts{},
			Setup: func(c *collector.Collector) {
				// higher backoff times because of strict cost rate limits
				c.SetPanicBackoff(
					2*time.Minute,
					5*time.Minute,
					10*time.Minute,
				)
			},
		},
		{
			Name:      "reservation",
			Config:    Config.Collectors.Reservation,
			Processor: &MetricsCollectorAzureRmReservation{},
		},
		{
			Name:      "budgets",
			Config:    Config.Collectors.Budgets,
			Processor: &MetricsCollectorAzureRmBudgets{},
		},
		{
			Name:      "advisor",
			Config:    Config.Collectors.Advisor,
			Processor: &MetricsCollectorAzureRmAdvisor{},
		},
		{
			Name:      "defender",
			Config:    Config.Collectors.Defender,
			Processor: &MetricsCollectorAzureRmDefender{},
		},
		{
			Name:      "resourceHealth",
			Config:    Config.Collectors.ResourceHealth,
			Processor: &MetricsCollectorAzureRmHealth{},
		},
		{
			Name:      "iam",
			Config:    Config.Collectors.Iam,
			Processor: &MetricsCollectorAzureRmIam{},
			Setup: func(c *collector.Collector) {
				initMsGraphConnection()
			},
		},
		{
			Name:      "graphApplications",
			Config:    Config.Collectors.Graph,
			Processor: &MetricsCollectorGraphApps{},
			Setup: func(c *collector.Collector) {
				initMsGraphConnection()
			},
		},
		{
			Name:      "graphServicePrincipals",
			Config:    Config.Collectors.Graph,
			Processor: &MetricsCollectorGraphServicePrincipals{},
			Setup: func(c *collector.Collector) {
				initMsGraphConnection()
			},
		},
		{
			Name:      "portscan",
			Config:    Config.Collectors.Portscan,
			Processor: &MetricsCollectorPortscanner{},
			Init: func() {
				// parse collectors.portscan.scanner.ports
				err := parseConfigPortScannerPortrange()
				if err != nil {
					logger.Fatal(err.Error())
				}
			},
		},
	}

	for _, collectorDefinition := range collectorList {
		collectorLogger := logger.With(slog.String("collector", collectorDefinition.Name))
		if collectorDefinition.Config.IsEnabled() {
			collectorLogger.Infof("collector enabled")

			if collectorDefinition.Init != nil {
				collectorDefinition.Init()
			}

			c := collector.New(collectorDefinition.Name, collectorDefinition.Processor, collectorLogger.Slog())
			c.SetScapeTime(collectorDefinition.Config.GetScrapeTime())
			if err := c.SetCache(
				Opts.GetCachePath(collectorDefinition.Name+".json"),
				collector.BuildCacheTag(cacheTag, Config.Azure, collectorDefinition.Config),
			); err != nil {
				collectorLogger.Fatal(err.Error())
			}

			if collectorDefinition.Setup != nil {
				collectorDefinition.Setup(c)
			}

			if err := c.Start(); err != nil {
				collectorLogger.Fatal(err.Error())
			}
		} else {
			collectorLogger.Infof("collector disabled")
		}
	}
}

// start and handle prometheus handler
func startHttpServer() {
	mux := http.NewServeMux()

	// healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err.Error())
		}
	})

	// readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err.Error())
		}
	})

	mux.Handle("/metrics", collector.HttpWaitForRlock(
		tracing.RegisterAzureMetricAutoClean(promhttp.Handler())),
	)

	srv := &http.Server{
		Addr:         Opts.Server.Bind,
		Handler:      mux,
		ReadTimeout:  Opts.Server.ReadTimeout,
		WriteTimeout: Opts.Server.WriteTimeout,
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal(err.Error())
	}
}
