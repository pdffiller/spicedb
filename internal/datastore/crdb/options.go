package crdb

import (
	"fmt"
	"time"

	pgxcommon "github.com/authzed/spicedb/internal/datastore/postgres/common"
)

type crdbOptions struct {
	readPoolOpts, writePoolOpts pgxcommon.PoolOptions

	watchBufferLength           uint16
	revisionQuantization        time.Duration
	followerReadDelay           time.Duration
	maxRevisionStalenessPercent float64
	gcWindow                    time.Duration
	maxRetries                  uint8
	splitAtUsersetCount         uint16
	overlapStrategy             string
	overlapKey                  string
	disableStats                bool

	enablePrometheusStats bool
}

const (
	errQuantizationTooLarge = "revision quantization (%s) must be less than GC window (%s)"

	overlapStrategyPrefix   = "prefix"
	overlapStrategyStatic   = "static"
	overlapStrategyInsecure = "insecure"

	defaultRevisionQuantization        = 5 * time.Second
	defaultFollowerReadDelay           = 0 * time.Second
	defaultMaxRevisionStalenessPercent = 0.1
	defaultWatchBufferLength           = 128
	defaultSplitSize                   = 1024

	defaultMaxRetries      = 5
	defaultOverlapKey      = "defaultsynckey"
	defaultOverlapStrategy = overlapStrategyStatic

	defaultEnablePrometheusStats = false
)

// Option provides the facility to configure how clients within the CRDB
// datastore interact with the running CockroachDB database.
type Option func(*crdbOptions)

func generateConfig(options []Option) (crdbOptions, error) {
	computed := crdbOptions{
		gcWindow:                    24 * time.Hour,
		watchBufferLength:           defaultWatchBufferLength,
		revisionQuantization:        defaultRevisionQuantization,
		followerReadDelay:           defaultFollowerReadDelay,
		maxRevisionStalenessPercent: defaultMaxRevisionStalenessPercent,
		splitAtUsersetCount:         defaultSplitSize,
		maxRetries:                  defaultMaxRetries,
		overlapKey:                  defaultOverlapKey,
		overlapStrategy:             defaultOverlapStrategy,
		disableStats:                false,
		enablePrometheusStats:       defaultEnablePrometheusStats,
	}

	for _, option := range options {
		option(&computed)
	}

	// Run any checks on the config that need to be done
	if computed.revisionQuantization >= computed.gcWindow {
		return computed, fmt.Errorf(
			errQuantizationTooLarge,
			computed.revisionQuantization,
			computed.gcWindow,
		)
	}

	return computed, nil
}

// SplitAtUsersetCount is the batch size for which userset queries will be
// split into smaller queries.
//
// This defaults to 1024.
func SplitAtUsersetCount(splitAtUsersetCount uint16) Option {
	return func(po *crdbOptions) {
		po.splitAtUsersetCount = splitAtUsersetCount
	}
}

// ReadConnHealthCheckInterval is the frequency at which both idle and max
// lifetime connections are checked, and also the frequency at which the
// minimum number of connections is checked.
//
// This happens asynchronously.
//
// This is not the only approach to evaluate these counts; "connection idle/max
// lifetime" is also checked when connections are released to the pool.
//
// There is no guarantee connections won't last longer than their specified
// idle/max lifetime. It's largely dependent on the health-check goroutine
// being able to pull them from the connection pool.
//
// The health-check may not be able to clean up those connections if they are
// held by the application very frequently.
//
// This value defaults to 30s.
func ReadConnHealthCheckInterval(interval time.Duration) Option {
	return func(po *crdbOptions) { po.readPoolOpts.ConnHealthCheckInterval = &interval }
}

// WriteConnHealthCheckInterval is the frequency at which both idle and max
// lifetime connections are checked, and also the frequency at which the
// minimum number of connections is checked.
//
// This happens asynchronously.
//
// This is not the only approach to evaluate these counts; "connection idle/max
// lifetime" is also checked when connections are released to the pool.
//
// There is no guarantee connections won't last longer than their specified
// idle/max lifetime. It's largely dependent on the health-check goroutine
// being able to pull them from the connection pool.
//
// The health-check may not be able to clean up those connections if they are
// held by the application very frequently.
//
// This value defaults to 30s.
func WriteConnHealthCheckInterval(interval time.Duration) Option {
	return func(po *crdbOptions) { po.writePoolOpts.ConnHealthCheckInterval = &interval }
}

// ReadConnMaxIdleTime is the duration after which an idle read connection will
// be automatically closed by the health check.
//
// This value defaults to having no maximum.
func ReadConnMaxIdleTime(idle time.Duration) Option {
	return func(po *crdbOptions) { po.readPoolOpts.ConnMaxIdleTime = &idle }
}

// WriteConnMaxIdleTime is the duration after which an idle write connection
// will be automatically closed by the health check.
//
// This value defaults to having no maximum.
func WriteConnMaxIdleTime(idle time.Duration) Option {
	return func(po *crdbOptions) { po.writePoolOpts.ConnMaxIdleTime = &idle }
}

// ReadConnMaxLifetime is the duration since creation after which a read
// connection will be automatically closed.
//
// This value defaults to having no maximum.
func ReadConnMaxLifetime(lifetime time.Duration) Option {
	return func(po *crdbOptions) { po.readPoolOpts.ConnMaxLifetime = &lifetime }
}

// ReadConnMaxLifetimeJitter is an interval to wait up to after the max lifetime
// to close the connection.
//
// This value defaults to 20% of the max lifetime.
func ReadConnMaxLifetimeJitter(jitter time.Duration) Option {
	return func(po *crdbOptions) { po.readPoolOpts.ConnMaxLifetimeJitter = &jitter }
}

// WriteConnMaxLifetime is the duration since creation after which a write
// connection will be automatically closed.
//
// This value defaults to having no maximum.
func WriteConnMaxLifetime(lifetime time.Duration) Option {
	return func(po *crdbOptions) { po.writePoolOpts.ConnMaxLifetime = &lifetime }
}

// WriteConnMaxLifetimeJitter is an interval to wait up to after the max lifetime
// to close the connection.
//
// This value defaults to 20% of the max lifetime.
func WriteConnMaxLifetimeJitter(jitter time.Duration) Option {
	return func(po *crdbOptions) { po.writePoolOpts.ConnMaxLifetimeJitter = &jitter }
}

// ReadConnsMinOpen is the minimum size of the connection pool used for reads.
//
// The health check will increase the number of connections to this amount if
// it had dropped below.
//
// This value defaults to the maximum open connections.
func ReadConnsMinOpen(conns int) Option {
	return func(po *crdbOptions) { po.readPoolOpts.MinOpenConns = &conns }
}

// WriteConnsMinOpen is the minimum size of the connection pool used for writes.
//
// The health check will increase the number of connections to this amount if
// it had dropped below.
//
// This value defaults to the maximum open connections.
func WriteConnsMinOpen(conns int) Option {
	return func(po *crdbOptions) { po.writePoolOpts.MinOpenConns = &conns }
}

// ReadConnsMaxOpen is the maximum size of the connection pool used for reads.
//
// This value defaults to having no maximum.
func ReadConnsMaxOpen(conns int) Option {
	return func(po *crdbOptions) { po.readPoolOpts.MaxOpenConns = &conns }
}

// WriteConnsMaxOpen is the maximum size of the connection pool used for writes.
//
// This value defaults to having no maximum.
func WriteConnsMaxOpen(conns int) Option {
	return func(po *crdbOptions) { po.writePoolOpts.MaxOpenConns = &conns }
}

// WatchBufferLength is the number of entries that can be stored in the watch
// buffer while awaiting read by the client.
//
// This value defaults to 128.
func WatchBufferLength(watchBufferLength uint16) Option {
	return func(po *crdbOptions) { po.watchBufferLength = watchBufferLength }
}

// RevisionQuantization is the time bucket size to which advertised revisions
// will be rounded.
//
// This value defaults to 5 seconds.
func RevisionQuantization(bucketSize time.Duration) Option {
	return func(po *crdbOptions) { po.revisionQuantization = bucketSize }
}

// FollowerReadDelay is the time delay to apply to enable historial reads.
//
// This value defaults to 0 seconds.
func FollowerReadDelay(delay time.Duration) Option {
	return func(po *crdbOptions) { po.followerReadDelay = delay }
}

// MaxRevisionStalenessPercent is the amount of time, expressed as a percentage of
// the revision quantization window, that a previously computed rounded revision
// can still be advertised after the next rounded revision would otherwise be ready.
//
// This value defaults to 0.1 (10%).
func MaxRevisionStalenessPercent(stalenessPercent float64) Option {
	return func(po *crdbOptions) { po.maxRevisionStalenessPercent = stalenessPercent }
}

// GCWindow is the maximum age of a passed revision that will be considered
// valid.
//
// This value defaults to 24 hours.
func GCWindow(window time.Duration) Option {
	return func(po *crdbOptions) { po.gcWindow = window }
}

// MaxRetries is the maximum number of times a retriable transaction will be
// client-side retried.
// Default: 5
func MaxRetries(maxRetries uint8) Option {
	return func(po *crdbOptions) { po.maxRetries = maxRetries }
}

// OverlapStrategy is the strategy used to generate overlap keys on write.
// Default: 'static'
func OverlapStrategy(strategy string) Option {
	return func(po *crdbOptions) { po.overlapStrategy = strategy }
}

// OverlapKey is a key touched on every write if OverlapStrategy is "static"
// Default: 'key'
func OverlapKey(key string) Option {
	return func(po *crdbOptions) { po.overlapKey = key }
}

// DisableStats disables recording counts to the stats table
func DisableStats(disable bool) Option {
	return func(po *crdbOptions) { po.disableStats = disable }
}

// WithEnablePrometheusStats marks whether Prometheus metrics provided by the Postgres
// clients being used by the datastore are enabled.
//
// Prometheus metrics are disabled by default.
func WithEnablePrometheusStats(enablePrometheusStats bool) Option {
	return func(po *crdbOptions) { po.enablePrometheusStats = enablePrometheusStats }
}
