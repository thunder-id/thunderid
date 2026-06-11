/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package service provides health check-related business logic and operations.
package service

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/healthcheck/model"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// HealthCheckServiceInterface defines the interface for the health check service.
type HealthCheckServiceInterface interface {
	CheckReadiness(ctx context.Context) model.ServerStatus
}

// HealthCheckService is the default implementation of the HealthCheckServiceInterface.
type HealthCheckService struct {
	DBProvider    provider.DBProviderInterface
	RedisProvider provider.RedisProviderInterface
}

// Initialize creates a new instance of HealthCheckService with the provided dependencies.
func Initialize(dbProvider provider.DBProviderInterface,
	redisProvider provider.RedisProviderInterface) HealthCheckServiceInterface {
	return &HealthCheckService{
		DBProvider:    dbProvider,
		RedisProvider: redisProvider,
	}
}

// CheckReadiness checks the readiness of the server and its dependencies.
func (hcs *HealthCheckService) CheckReadiness(ctx context.Context) model.ServerStatus {
	configDBStatus := model.ServiceStatus{
		ServiceName: "ConfigDB",
		Status:      hcs.checkConfigDatabaseStatus(ctx, queryConfigDBTable),
	}

	runtimeDBStatus := model.ServiceStatus{
		ServiceName: "RuntimeDB",
		Status:      hcs.checkRuntimeDatabaseStatus(ctx, queryRuntimeDBTable),
	}

	userDBStatus := model.ServiceStatus{
		ServiceName: "UserDB",
		Status:      hcs.checkUserDatabaseStatus(ctx, queryUserDBTable),
	}

	status := model.StatusUp
	if configDBStatus.Status == model.StatusDown ||
		runtimeDBStatus.Status == model.StatusDown ||
		userDBStatus.Status == model.StatusDown {
		status = model.StatusDown
	}
	return model.ServerStatus{
		Status: status,
		ServiceStatus: []model.ServiceStatus{
			configDBStatus,
			runtimeDBStatus,
			userDBStatus,
		},
	}
}

// checkConfigDatabaseStatus checks the status of the config database with the specified query.
func (hcs *HealthCheckService) checkConfigDatabaseStatus(ctx context.Context, query dbmodel.DBQuery) model.Status {
	dbClient, err := hcs.DBProvider.GetConfigDBClient()
	return hcs.executeDatabaseHealthCheck(ctx, "ConfigDB", dbClient, err, query)
}

// checkRuntimeDatabaseStatus checks the status of the runtime database with the specified query.
func (hcs *HealthCheckService) checkRuntimeDatabaseStatus(ctx context.Context, query dbmodel.DBQuery) model.Status {
	if config.GetServerRuntime().Config.Database.Runtime.Type == provider.DataSourceTypeRedis {
		return hcs.checkRedisRuntimeStatus(ctx)
	}
	dbClient, err := hcs.DBProvider.GetRuntimeDBClient()
	return hcs.executeDatabaseHealthCheck(ctx, "RuntimeDB", dbClient, err, query)
}

// checkRedisRuntimeStatus checks the health of the Redis runtime store via Ping.
func (hcs *HealthCheckService) checkRedisRuntimeStatus(ctx context.Context) model.Status {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "HealthCheckService"))
	if hcs.RedisProvider == nil {
		logger.Error(ctx, "Redis runtime provider is not initialized")
		return model.StatusDown
	}
	if err := hcs.RedisProvider.GetRedisClient().Ping(context.Background()).Err(); err != nil {
		logger.Error(ctx, "Failed to ping Redis runtime store", log.Error(err))
		return model.StatusDown
	}
	return model.StatusUp
}

// checkUserDatabaseStatus checks the status of the runtime database with the specified query.
func (hcs *HealthCheckService) checkUserDatabaseStatus(ctx context.Context, query dbmodel.DBQuery) model.Status {
	dbClient, err := hcs.DBProvider.GetUserDBClient()
	return hcs.executeDatabaseHealthCheck(ctx, "UserDB", dbClient, err, query)
}

// executeDatabaseHealthCheck runs the provided query on the given database client and reports its status.
func (hcs *HealthCheckService) executeDatabaseHealthCheck(ctx context.Context,
	dbName string, dbClient provider.DBClientInterface, err error, query dbmodel.DBQuery,
) model.Status {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "HealthCheckService"))

	if err != nil {
		logger.Error(ctx, "Failed to get database client", log.String("dbname", dbName), log.Error(err))
		return model.StatusDown
	}

	_, err = dbClient.Query(query)
	if err != nil {
		logger.Error(ctx, "Failed to execute query", log.String("dbname", dbName), log.Error(err))
		return model.StatusDown
	}
	return model.StatusUp
}
