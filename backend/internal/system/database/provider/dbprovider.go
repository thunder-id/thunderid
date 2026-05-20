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

// Package provider provides functionality for managing database connections and clients.
package provider

import (
	"database/sql"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

const (
	dataSourceTypePostgres = "postgres"
	dataSourceTypeSQLite   = "sqlite"

	dbNameConfig  = "config"
	dbNameRuntime = "runtime"
	dbNameUser    = "user"
)

// dbConfig represents the local database configuration.
type dbConfig struct {
	dsn        string
	driverName string
}

// DBProviderInterface defines the interface for getting database clients and transactioners.
type DBProviderInterface interface {
	GetConfigDBClient() (DBClientInterface, error)
	GetRuntimeDBClient() (DBClientInterface, error)
	GetUserDBClient() (DBClientInterface, error)
	GetConfigDBTransactioner() (transaction.Transactioner, error)
	GetUserDBTransactioner() (transaction.Transactioner, error)
	GetRuntimeDBTransactioner() (transaction.Transactioner, error)
}

// DBProviderCloser is a separate interface for closing the provider.
// Only the lifecycle manager should use this interface.
type DBProviderCloser interface {
	Close() error
}

// dbProvider is the implementation of DBProviderInterface.
type dbProvider struct {
	configClient  DBClientInterface
	configMutex   sync.RWMutex
	runtimeClient DBClientInterface
	runtimeMutex  sync.RWMutex
	userClient    DBClientInterface
	userMutex     sync.RWMutex
}

var (
	instance *dbProvider
	once     sync.Once
)

// initDBProvider initializes the singleton instance of DBProvider.
func initDBProvider() {
	once.Do(func() {
		instance = &dbProvider{}
		instance.initializeAllClients()
	})
}

// GetDBProvider returns the instance of DBProvider.
func GetDBProvider() DBProviderInterface {
	initDBProvider()
	return instance
}

// GetDBProviderCloser returns the DBProvider with closing capability.
// This should only be called from the main lifecycle manager.
func GetDBProviderCloser() DBProviderCloser {
	initDBProvider()
	return instance
}

// GetConfigDBClient returns a database client for config datasource.
// Not required to close the returned client manually since it manages its own connection pool.
func (d *dbProvider) GetConfigDBClient() (DBClientInterface, error) {
	configDBConfig := config.GetServerRuntime().Config.Database.Config
	return d.getOrInitClient(&d.configClient, &d.configMutex, configDBConfig, dbNameConfig)
}

// GetRuntimeDBClient returns a database client for runtime datasource.
// Not required to close the returned client manually since it manages its own connection pool.
func (d *dbProvider) GetRuntimeDBClient() (DBClientInterface, error) {
	runtimeDBConfig := config.GetServerRuntime().Config.Database.Runtime
	return d.getOrInitClient(&d.runtimeClient, &d.runtimeMutex, runtimeDBConfig, dbNameRuntime)
}

// GetUserDBClient returns a database client for runtime datasource.
// Not required to close the returned client manually since it manages its own connection pool.
func (d *dbProvider) GetUserDBClient() (DBClientInterface, error) {
	userDBConfig := config.GetServerRuntime().Config.Database.User
	return d.getOrInitClient(&d.userClient, &d.userMutex, userDBConfig, dbNameUser)
}

// GetConfigDBTransactioner returns a transactioner for the config database.
// The transactioner manages database transactions with automatic nesting detection.
func (d *dbProvider) GetConfigDBTransactioner() (transaction.Transactioner, error) {
	return d.getTransactioner(d.GetConfigDBClient, dbNameConfig)
}

// GetUserDBTransactioner returns a transactioner for the user database.
// The transactioner manages database transactions with automatic nesting detection.
func (d *dbProvider) GetUserDBTransactioner() (transaction.Transactioner, error) {
	return d.getTransactioner(d.GetUserDBClient, dbNameUser)
}

// GetRuntimeDBTransactioner returns a transactioner for the runtime database.
func (d *dbProvider) GetRuntimeDBTransactioner() (transaction.Transactioner, error) {
	// When the runtime store is Redis, a no-op transactioner is returned since Redis does
	// not support SQL-style transactions.
	if config.GetServerRuntime().Config.Database.Runtime.Type == DataSourceTypeRedis {
		return transaction.NewNoOpTransactioner(), nil
	}
	return d.getTransactioner(d.GetRuntimeDBClient, dbNameRuntime)
}

// getTransactioner is a helper method that creates a transactioner for a given database client.
func (d *dbProvider) getTransactioner(
	clientGetter func() (DBClientInterface, error),
	dbName string,
) (transaction.Transactioner, error) {
	client, err := clientGetter()
	if err != nil {
		return nil, fmt.Errorf("failed to get %s database client: %w", dbName, err)
	}

	return client.GetTransactioner()
}

// initializeAllClients initializes config, runtime, and user database clients at startup.
func (d *dbProvider) initializeAllClients() {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DBProvider"))

	configDBConfig := config.GetServerRuntime().Config.Database.Config
	err := d.initializeClient(&d.configClient, configDBConfig, dbNameConfig)
	if err != nil {
		logger.Error("Failed to initialize config database client", log.Error(err))
	}

	runtimeDBConfig := config.GetServerRuntime().Config.Database.Runtime
	if runtimeDBConfig.Type != DataSourceTypeRedis {
		err = d.initializeClient(&d.runtimeClient, runtimeDBConfig, dbNameRuntime)
		if err != nil {
			logger.Error("Failed to initialize runtime database client", log.Error(err))
		}
	}

	userDBConfig := config.GetServerRuntime().Config.Database.User
	err = d.initializeClient(&d.userClient, userDBConfig, dbNameUser)
	if err != nil {
		logger.Error("Failed to initialize user database client", log.Error(err))
	}
}

// getOrInitClient gets or initializes a DB client with locking.
func (d *dbProvider) getOrInitClient(
	clientPtr *DBClientInterface,
	mutex *sync.RWMutex,
	dataSource config.DataSource,
	dbName string,
) (DBClientInterface, error) {
	// Return error if database type is not configured
	if dataSource.Type == "" {
		return nil, fmt.Errorf("database type is not configured")
	}
	// Redis runtime stores bypass the SQL client entirely
	if dataSource.Type == DataSourceTypeRedis {
		return nil, fmt.Errorf("runtime database is configured as Redis; use RedisProvider instead")
	}

	mutex.RLock()
	if *clientPtr != nil {
		client := *clientPtr
		mutex.RUnlock()
		return client, nil
	}
	mutex.RUnlock()

	mutex.Lock()
	defer mutex.Unlock()

	if *clientPtr != nil {
		return *clientPtr, nil
	}

	if err := d.initializeClient(clientPtr, dataSource, dbName); err != nil {
		return nil, err
	}

	return *clientPtr, nil
}

// initializeClient initializes a database client and assigns it to the provided pointer.
func (d *dbProvider) initializeClient(clientPtr *DBClientInterface, dataSource config.DataSource, dbName string) error {
	dbConfig := d.getDBConfig(dataSource)

	db, err := sql.Open(dbConfig.driverName, dbConfig.dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database %s: %w", dbName, err)
	}

	// Configure connection pool using values from the type-specific sub-config.
	var maxOpenConns, maxIdleConns, connMaxLifetime int
	switch dataSource.Type {
	case dataSourceTypePostgres:
		maxOpenConns = dataSource.Postgres.MaxOpenConns
		maxIdleConns = dataSource.Postgres.MaxIdleConns
		connMaxLifetime = dataSource.Postgres.ConnMaxLifetime
	case dataSourceTypeSQLite:
		maxOpenConns = dataSource.SQLite.MaxOpenConns
		maxIdleConns = dataSource.SQLite.MaxIdleConns
		connMaxLifetime = dataSource.SQLite.ConnMaxLifetime
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(time.Duration(connMaxLifetime) * time.Second)

	// Test the database connection.
	if err := db.Ping(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return fmt.Errorf("failed to ping database %s: %w (close error: %w)", dbName, err, closeErr)
		}
		return fmt.Errorf("failed to ping database %s: %w", dbName, err)
	}

	// Enable foreign key constraints for SQLite databases
	if dbConfig.driverName == dataSourceTypeSQLite {
		_, err := db.Exec("PRAGMA foreign_keys = ON;")
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				return fmt.Errorf("failed to enable foreign key constraints for %s: %w (close error: %w)",
					dbName, err, closeErr)
			}
			return fmt.Errorf("failed to enable foreign key constraints for %s: %w", dbName, err)
		}
	}

	var rc retryConfig
	switch dataSource.Type {
	case dataSourceTypePostgres:
		rc = retryConfig{
			MaxAttempts: dataSource.Postgres.MaxRetries,
			MinBackoff:  time.Duration(dataSource.Postgres.MinRetryBackoffMS) * time.Millisecond,
			MaxBackoff:  time.Duration(dataSource.Postgres.MaxRetryBackoffMS) * time.Millisecond,
		}
	case dataSourceTypeSQLite:
		rc = retryConfig{
			MaxAttempts: dataSource.SQLite.MaxRetries,
			MinBackoff:  time.Duration(dataSource.SQLite.MinRetryBackoffMS) * time.Millisecond,
			MaxBackoff:  time.Duration(dataSource.SQLite.MaxRetryBackoffMS) * time.Millisecond,
		}
	}

	*clientPtr = NewDBClient(model.NewDB(db), dbConfig.driverName, dbName, rc)
	return nil
}

// getDBConfig returns the database configuration based on the provided data source.
func (d *dbProvider) getDBConfig(dataSource config.DataSource) dbConfig {
	var dbConfig dbConfig

	switch dataSource.Type {
	case dataSourceTypePostgres:
		pg := dataSource.Postgres
		dbConfig.driverName = dataSourceTypePostgres
		dbConfig.dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			pg.Hostname, pg.Port, pg.Username, pg.Password, pg.Name, pg.SSLMode)
	case dataSourceTypeSQLite:
		sl := dataSource.SQLite
		dbConfig.driverName = dataSourceTypeSQLite
		options := sl.Options
		if options != "" && options[0] != '?' {
			options = "?" + options
		}
		dbConfig.dsn = fmt.Sprintf("%s%s", path.Join(config.GetServerRuntime().ServerHome, sl.Path), options)
	}

	return dbConfig
}

// Close closes the database connections. This should only be called by the lifecycle manager during shutdown.
func (d *dbProvider) Close() error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DBProvider"))
	logger.Debug("Closing database connections")

	configErr := d.closeClient(&d.configClient, &d.configMutex, "config")
	runtimeErr := d.closeClient(&d.runtimeClient, &d.runtimeMutex, "runtime")
	userErr := d.closeClient(&d.userClient, &d.userMutex, "user")

	// Close the Redis runtime provider if it was initialized.
	var redisErr error
	if redisInstance != nil {
		redisErr = redisInstance.Close()
	}

	return errors.Join(configErr, runtimeErr, userErr, redisErr)
}

// closeClient is a helper to close a DB client with locking.
func (d *dbProvider) closeClient(clientPtr *DBClientInterface, mutex *sync.RWMutex, clientName string) error {
	mutex.Lock()
	defer mutex.Unlock()
	if *clientPtr != nil {
		if client, ok := (*clientPtr).(*DBClient); ok {
			if err := client.close(); err != nil {
				return fmt.Errorf("failed to close %s client: %w", clientName, err)
			}
		}
		*clientPtr = nil
	}
	return nil
}
