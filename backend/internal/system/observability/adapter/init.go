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

package adapter

import (
	"github.com/IBM/sarama"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// InitializeConsoleAdapter creates and returns a console adapter that writes formatted events to stdout.
//
// Returns:
//   - OutputAdapterInterface: The initialized console adapter instance
//
// Example:
//
//	adapter := adapter.InitializeConsoleAdapter()
//	err := adapter.Write(formattedData)
func InitializeConsoleAdapter() OutputAdapterInterface {
	return newConsoleAdapter()
}

// InitializeFileAdapter creates and returns a file adapter that writes formatted events to a file
// with optional rotation support.
//
// Parameters:
//   - filePath: The path to the file where events will be written
//
// Returns:
//   - OutputAdapterInterface: The initialized file adapter instance
//   - error: Error if the adapter cannot be created (e.g., invalid path, permission issues)
//
// Example:
//
//	adapter, err := adapter.InitializeFileAdapter("/var/log/observability/events.log")
//	if err != nil {
//	    return err
//	}
//	err = adapter.Write(formattedData)
func InitializeFileAdapter(filePath string) (OutputAdapterInterface, error) {
	return NewFileAdapter(filePath)
}

// InitializeKafkaAdapter creates a kafka adapter that publishes formatted events to a Kafka topic
// using an asynchronous sarama producer.
//
// Parameters:
//   - cfg: Kafka sink configuration (brokers, topic, retries, timeout, client id)
//
// Returns:
//   - OutputAdapterInterface: The initialized kafka adapter instance
//   - error: Error if broker connection or producer creation fails
func InitializeKafkaAdapter(cfg config.ObservabilityKafkaConfig) (OutputAdapterInterface, error) {
	return newKafkaAdapter(cfg, sarama.NewAsyncProducer)
}
