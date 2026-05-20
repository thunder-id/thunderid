/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	dbRetryMaxAttempts = 3
	dbRetryMinBackoff  = 50 * time.Millisecond
	dbRetryMaxBackoff  = 2 * time.Second
	dbRetryRandFloat64 = rand.Float64
)

type retryConfig struct {
	MaxAttempts int
	MinBackoff  time.Duration
	MaxBackoff  time.Duration
	RandFloat64 func() float64
}

type dbRetryMetrics struct {
	once             sync.Once
	retryAttempts    metric.Int64Counter
	retryBackoff     metric.Float64Histogram
	operationLatency metric.Float64Histogram
}

var retryMetrics dbRetryMetrics

func initDBRetryMetrics() {
	retryMetrics.once.Do(func() {
		meter := otel.Meter("github.com/thunder-id/thunderid/database/retry")
		retryMetrics.retryAttempts, _ = meter.Int64Counter(
			"thunderid_db_retry_attempts_total",
			metric.WithDescription("Total DB retry attempts for transient errors"),
		)
		retryMetrics.retryBackoff, _ = meter.Float64Histogram(
			"thunderid_db_retry_backoff_seconds",
			metric.WithDescription("Backoff delay used before DB retry attempts"),
		)
		retryMetrics.operationLatency, _ = meter.Float64Histogram(
			"thunderid_db_operation_seconds",
			metric.WithDescription("Latency of DB operations executed through retry wrapper"),
		)
	})
}

func withRetryDB(
	ctx context.Context,
	dbType, dbName, queryID string,
	retryConfig retryConfig,
	fn func(context.Context) error,
) error {
	config := normalizeRetryConfig(retryConfig)
	if config.MaxAttempts <= 0 {
		return fn(ctx)
	}

	initDBRetryMetrics()
	start := time.Now()
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := fn(ctx)
		if err == nil {
			recordDBOperationLatency(ctx, dbType, dbName, queryID, "success", time.Since(start))
			return nil
		}

		lastErr = err

		if errors.Is(err, context.Canceled) {
			recordDBOperationLatency(ctx, dbType, dbName, queryID, "cancelled", time.Since(start))
			return err
		}

		if !isRetryableDBError(err) {
			recordDBOperationLatency(ctx, dbType, dbName, queryID, "failed", time.Since(start))
			return err
		}

		if attempt == config.MaxAttempts {
			break
		}

		delay := calculateRetryDelay(attempt, config)
		recordDBRetryAttempt(ctx, dbType, dbName, queryID, attempt, delay)

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			recordDBOperationLatency(ctx, dbType, dbName, queryID, "cancelled", time.Since(start))
			return ctx.Err()
		case <-timer.C:
		}
	}

	recordDBOperationLatency(ctx, dbType, dbName, queryID, "failed", time.Since(start))
	return fmt.Errorf("database retry attempts exhausted: %w", lastErr)
}

func normalizeRetryConfig(config retryConfig) retryConfig {
	if config.MaxAttempts == 0 {
		config.MaxAttempts = dbRetryMaxAttempts
	}
	if config.MinBackoff <= 0 {
		config.MinBackoff = dbRetryMinBackoff
	}
	if config.MaxBackoff <= 0 {
		config.MaxBackoff = dbRetryMaxBackoff
	}
	if config.MaxBackoff < config.MinBackoff {
		config.MaxBackoff = config.MinBackoff
	}
	if config.RandFloat64 == nil {
		config.RandFloat64 = dbRetryRandFloat64
	}

	return config
}

func calculateRetryDelay(attempt int, config retryConfig) time.Duration {
	if attempt <= 0 {
		return config.MinBackoff
	}

	exponentialFactor := math.Pow(2, float64(attempt-1))
	base := time.Duration(float64(config.MinBackoff) * exponentialFactor)
	if base > config.MaxBackoff {
		base = config.MaxBackoff
	}

	jitter := time.Duration(config.RandFloat64() * float64(base))
	delay := base + jitter
	if delay > config.MaxBackoff {
		return config.MaxBackoff
	}

	return delay
}

func isRetryableDBError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) ||
		errors.Is(err, sql.ErrNoRows) ||
		errors.Is(err, sql.ErrTxDone) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, sql.ErrConnDone) {
		return true
	}

	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		code := string(pqErr.Code)
		if code == "40P01" ||
			strings.HasPrefix(code, "53") ||
			code == "57P01" ||
			code == "57P02" ||
			code == "57P03" {
			return true
		}
	}

	if isTransientNetworkError(err) {
		return true
	}

	return false
}

func isTransientNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	if hasSyscallErrno(err,
		syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.EPIPE,
		syscall.ETIMEDOUT,
		syscall.ECONNABORTED,
		syscall.EHOSTUNREACH,
		syscall.ENETUNREACH,
	) {
		return true
	}

	return false
}

func hasSyscallErrno(err error, allowed ...syscall.Errno) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if hasSyscallErrno(opErr.Err, allowed...) {
			return true
		}
	}

	var sysErr *os.SyscallError
	if errors.As(err, &sysErr) {
		if hasSyscallErrno(sysErr.Err, allowed...) {
			return true
		}
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		for _, candidate := range allowed {
			if errno == candidate {
				return true
			}
		}
	}

	return false
}

func recordDBRetryAttempt(
	ctx context.Context,
	dbType, dbName, queryID string,
	attempt int,
	delay time.Duration,
) {
	attrs := metric.WithAttributes(
		attribute.String("db.type", dbType),
		attribute.String("db.name", dbName),
		attribute.String("db.query_id", queryID),
		attribute.Int("db.retry_attempt", attempt),
	)
	if retryMetrics.retryAttempts != nil {
		retryMetrics.retryAttempts.Add(ctx, 1, attrs)
	}
	if retryMetrics.retryBackoff != nil {
		retryMetrics.retryBackoff.Record(ctx, delay.Seconds(), attrs)
	}
}

func recordDBOperationLatency(
	ctx context.Context,
	dbType, dbName, queryID, status string,
	duration time.Duration,
) {
	if retryMetrics.operationLatency == nil {
		return
	}
	retryMetrics.operationLatency.Record(
		ctx,
		duration.Seconds(),
		metric.WithAttributes(
			attribute.String("db.type", dbType),
			attribute.String("db.name", dbName),
			attribute.String("db.query_id", queryID),
			attribute.String("db.status", status),
		),
	)
}
