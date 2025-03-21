package overrides

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/services"
	"github.com/grafana/dskit/tracing"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v2"

	userconfigurableoverrides "github.com/grafana/tempo/modules/overrides/userconfigurable/client"
	tempo_log "github.com/grafana/tempo/pkg/util/log"
	"github.com/grafana/tempo/tempodb/backend"
)

type UserConfigurableOverridesConfig struct {
	Enabled bool `yaml:"enabled"`

	// PollInterval controls how often the overrides will be refreshed by polling the backend
	PollInterval time.Duration `yaml:"poll_interval"`

	Client userconfigurableoverrides.Config `yaml:"client"`
}

func (cfg *UserConfigurableOverridesConfig) RegisterFlagsAndApplyDefaults(f *flag.FlagSet) {
	cfg.PollInterval = time.Minute

	cfg.Client.RegisterFlagsAndApplyDefaults(f)
}

type tenantLimits map[string]*userconfigurableoverrides.Limits

// userConfigurableOverridesManager can store user-configurable overrides on a bucket.
type userConfigurableOverridesManager struct {
	services.Service
	// wrap Interface and only overrides select functions
	Interface

	cfg *UserConfigurableOverridesConfig

	subservices        *services.Manager
	subservicesWatcher *services.FailureWatcher

	mtx          sync.RWMutex
	tenantLimits tenantLimits

	client userconfigurableoverrides.Client

	logger log.Logger
}

var _ Service = (*userConfigurableOverridesManager)(nil)

// newUserConfigOverrides wraps the given overrides with user-configurable overrides.
func newUserConfigOverrides(cfg *UserConfigurableOverridesConfig, subOverrides Service) (*userConfigurableOverridesManager, error) {
	client, err := userconfigurableoverrides.New(&cfg.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize backend client for user-configurable overrides: %w", err)
	}

	mgr := userConfigurableOverridesManager{
		Interface:    subOverrides,
		cfg:          cfg,
		tenantLimits: make(tenantLimits),
		client:       client,
		logger:       log.With(tempo_log.Logger, "component", "user-configurable overrides"),
	}

	mgr.subservices, err = services.NewManager(subOverrides)
	if err != nil {
		return nil, fmt.Errorf("failed to create subservices: %w", err)
	}
	mgr.subservicesWatcher = services.NewFailureWatcher()
	mgr.subservicesWatcher.WatchManager(mgr.subservices)

	mgr.Service = services.NewBasicService(mgr.starting, mgr.running, mgr.stopping)

	return &mgr, nil
}

func (o *userConfigurableOverridesManager) starting(ctx context.Context) error {
	if err := services.StartManagerAndAwaitHealthy(ctx, o.subservices); err != nil {
		return errors.Wrap(err, "unable to start overrides subservices")
	}

	return o.reloadAllTenantLimits(ctx)
}

func (o *userConfigurableOverridesManager) running(ctx context.Context) error {
	ticker := time.NewTicker(o.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			err := o.reloadAllTenantLimits(ctx)
			if err != nil {
				level.Error(o.logger).Log("msg", "failed to refresh user-configurable config", "err", err)
			}
			continue

		case err := <-o.subservicesWatcher.Chan():
			return errors.Wrap(err, "overrides subservice failed")
		}
	}
}

func (o *userConfigurableOverridesManager) stopping(error) error {
	return services.StopManagerAndAwaitStopped(context.Background(), o.subservices)
}

func (o *userConfigurableOverridesManager) reloadAllTenantLimits(ctx context.Context) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "userConfigurableOverridesManager.reloadAllTenantLimits")
	defer span.Finish()

	traceID, _ := tracing.ExtractTraceID(ctx)
	level.Info(o.logger).Log("msg", "reloading all tenant limits", "traceID", traceID)

	// List tenants with user-configurable overrides
	tenants, err := o.client.List(ctx)
	if err != nil {
		return err
	}

	// Clean up cached tenants that have been removed from the backend
	for cachedTenant := range o.tenantLimits {
		if !slices.Contains(tenants, cachedTenant) {
			o.setTenantLimit(cachedTenant, nil)
		}
	}

	// For every tenant with user-configurable overrides, download and cache them
	for _, tenant := range tenants {
		limits, _, err := o.client.Get(ctx, tenant)
		if err == backend.ErrDoesNotExist {
			o.setTenantLimit(tenant, nil)
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to load tenant limits for tenant %v", tenant)
		}
		o.setTenantLimit(tenant, limits)
	}

	return nil
}

// getTenantLimits returns the tenant limits for the given tenant, can be nil.
func (o *userConfigurableOverridesManager) getTenantLimits(userID string) *userconfigurableoverrides.Limits {
	o.mtx.RLock()
	defer o.mtx.RUnlock()

	return o.tenantLimits[userID]
}

func (o *userConfigurableOverridesManager) getAllTenantLimits() tenantLimits {
	o.mtx.RLock()
	defer o.mtx.RUnlock()

	return o.tenantLimits
}

func (o *userConfigurableOverridesManager) setTenantLimit(userID string, limits *userconfigurableoverrides.Limits) {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	if limits == nil {
		delete(o.tenantLimits, userID)
	} else {
		o.tenantLimits[userID] = limits
	}
}

func (o *userConfigurableOverridesManager) Forwarders(userID string) []string {
	if forwarders, ok := o.getTenantLimits(userID).GetForwarders(); ok {
		return forwarders
	}
	return o.Interface.Forwarders(userID)
}

func (o *userConfigurableOverridesManager) MetricsGeneratorProcessors(userID string) map[string]struct{} {
	if processors, ok := o.getTenantLimits(userID).GetMetricsGenerator().GetProcessors(); ok {
		return processors.GetMap()
	}
	return o.Interface.MetricsGeneratorProcessors(userID)
}

func (o *userConfigurableOverridesManager) MetricsGeneratorDisableCollection(userID string) bool {
	if disableCollection, ok := o.getTenantLimits(userID).GetMetricsGenerator().GetDisableCollection(); ok {
		return disableCollection
	}
	return o.Interface.MetricsGeneratorDisableCollection(userID)
}

func (o *userConfigurableOverridesManager) MetricsGeneratorProcessorServiceGraphsDimensions(userID string) []string {
	if dimensions, ok := o.getTenantLimits(userID).GetMetricsGenerator().GetProcessor().GetServiceGraphs().GetDimensions(); ok {
		return dimensions
	}
	return o.Interface.MetricsGeneratorProcessorServiceGraphsDimensions(userID)
}

func (o *userConfigurableOverridesManager) MetricsGeneratorProcessorServiceGraphsEnableClientServerPrefix(userID string) bool {
	if enableClientServerPrefix, ok := o.getTenantLimits(userID).GetMetricsGenerator().GetProcessor().GetServiceGraphs().GetEnableClientServerPrefix(); ok {
		return enableClientServerPrefix
	}
	return o.Interface.MetricsGeneratorProcessorServiceGraphsEnableClientServerPrefix(userID)
}

func (o *userConfigurableOverridesManager) MetricsGeneratorProcessorServiceGraphsPeerAttributes(userID string) []string {
	if peerAttribtues, ok := o.getTenantLimits(userID).GetMetricsGenerator().GetProcessor().GetServiceGraphs().GetPeerAttributes(); ok {
		return peerAttribtues
	}
	return o.Interface.MetricsGeneratorProcessorServiceGraphsPeerAttributes(userID)
}

func (o *userConfigurableOverridesManager) MetricsGeneratorProcessorSpanMetricsDimensions(userID string) []string {
	if dimensions, ok := o.getTenantLimits(userID).GetMetricsGenerator().GetProcessor().GetSpanMetrics().GetDimensions(); ok {
		return dimensions
	}
	return o.Interface.MetricsGeneratorProcessorSpanMetricsDimensions(userID)
}

func (o *userConfigurableOverridesManager) MetricsGeneratorProcessorSpanMetricsEnableTargetInfo(userID string) bool {
	if enableTargetInfo, ok := o.getTenantLimits(userID).GetMetricsGenerator().GetProcessor().GetSpanMetrics().GetEnableTargetInfo(); ok {
		return enableTargetInfo
	}
	return o.Interface.MetricsGeneratorProcessorSpanMetricsEnableTargetInfo(userID)
}

// statusUserConfigurableOverrides used to marshal userconfigurableoverrides.Limits for tenants
type statusUserConfigurableOverrides struct {
	TenantLimits tenantLimits `yaml:"user_configurable_overrides" json:"user_configurable_overrides"`
}

func (o *userConfigurableOverridesManager) WriteStatusRuntimeConfig(w io.Writer, r *http.Request) error {
	// fetch runtimeConfig and Runtime per tenant Overrides
	err := o.Interface.WriteStatusRuntimeConfig(w, r)
	if err != nil {
		return err
	}

	// now write per tenant user configured overrides
	// wrap in userConfigOverrides struct to return correct yaml
	l := o.getAllTenantLimits()
	ucl := statusUserConfigurableOverrides{TenantLimits: l}
	out, err := yaml.Marshal(ucl)
	if err != nil {
		return err
	}

	_, err = w.Write(out)
	if err != nil {
		return err
	}

	return nil
}

func (o *userConfigurableOverridesManager) Describe(ch chan<- *prometheus.Desc) {
	// TODO for now just pass along to the underlying overrides, in the future we should export
	//  the user-config overrides as well
	o.Interface.Describe(ch)
}

func (o *userConfigurableOverridesManager) Collect(ch chan<- prometheus.Metric) {
	// TODO for now just pass along to the underlying overrides, in the future we should export
	//  the user-config overrides as well
	o.Interface.Collect(ch)
}
