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

	err = yaml.UnmarshalWithOptions(defaultConfig, &Config, yaml.Strict())
	if err != nil {
		logger.Fatal(err.Error())
	}

	logger.Infof(`reading config from "%v"`, Opts.Config)
	/* #nosec */
	content, err := os.ReadFile(Opts.Config)
	if err != nil {
		logger.Fatal(err.Error())
	}

	err = yaml.UnmarshalWithOptions(content, &Config, yaml.Strict())
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

func initMetricCollector() {
	var collectorName string

	collectorName = "general"
	if Config.Collectors.General.IsEnabled() {
		c := collector.New(collectorName, &MetricsCollectorAzureRmGeneral{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.General.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.General),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	}

	collectorName = "resource"
	if Config.Collectors.Resource.IsEnabled() {
		c := collector.New(collectorName, &MetricsCollectorAzureRmResources{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Resource.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Resource),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "quota"
	if Config.Collectors.Quota.IsEnabled() {
		c := collector.New(collectorName, &MetricsCollectorAzureRmQuota{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Quota.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Quota),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "costs"
	if Config.Collectors.Costs.IsEnabled() {
		c := collector.New(collectorName, &MetricsCollectorAzureRmCosts{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Costs.ScrapeTime)
		// higher backoff times because of strict cost rate limits
		c.SetPanicBackoff(
			2*time.Minute,
			5*time.Minute,
			10*time.Minute,
		)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Costs),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "reservation"
	if Config.Collectors.Reservation.IsEnabled() {
		c := collector.New(collectorName, &MetricsCollectorAzureRmReservation{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Reservation.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Reservation),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "budgets"
	if Config.Collectors.Budgets.IsEnabled() {
		c := collector.New(collectorName, &MetricsCollectorAzureRmBudgets{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Budgets.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Budgets),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "defender"
	if Config.Collectors.Defender.IsEnabled() {
		c := collector.New(collectorName, &MetricsCollectorAzureRmDefender{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Defender.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Defender),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "resourceHealth"
	if Config.Collectors.ResourceHealth.IsEnabled() {
		c := collector.New(collectorName, &MetricsCollectorAzureRmHealth{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.ResourceHealth.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.ResourceHealth),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "iam"
	if Config.Collectors.Iam.IsEnabled() {
		initMsGraphConnection()
		c := collector.New(collectorName, &MetricsCollectorAzureRmIam{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Iam.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Iam, Opts.Azure.Tenant),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "graphApplications"
	if Config.Collectors.Graph.IsEnabled() {
		initMsGraphConnection()
		c := collector.New(collectorName, &MetricsCollectorGraphApps{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Graph.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Graph, Opts.Azure.Tenant),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "graphServicePrincipals"
	if Config.Collectors.Graph.IsEnabled() {
		initMsGraphConnection()
		c := collector.New(collectorName, &MetricsCollectorGraphServicePrincipals{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Graph.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Graph, Opts.Azure.Tenant),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Panic(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
	}

	collectorName = "portscan"
	if Config.Collectors.Portscan.IsEnabled() {
		// parse collectors.portscan.scanner.ports
		err := parseConfigPortScannerPortrange()
		if err != nil {
			logger.Fatal(err.Error())
		}

		c := collector.New(collectorName, &MetricsCollectorPortscanner{}, logger.Slog())
		c.SetScapeTime(*Config.Collectors.Portscan.ScrapeTime)
		if err := c.SetCache(
			Opts.GetCachePath(collectorName+".json"),
			collector.BuildCacheTag(cacheTag, Config.Azure, Config.Collectors.Portscan),
		); err != nil {
			logger.Fatal(err.Error())
		}
		if err := c.Start(); err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.With(slog.String("collector", collectorName)).Infof("collector disabled")
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
