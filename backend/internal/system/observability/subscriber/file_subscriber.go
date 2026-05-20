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

package subscriber

import (
	"fmt"
	"path/filepath"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/adapter"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/formatter"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const fileSubscriberComponentName = "FileSubscriber"

// FileSubscriber writes observability events to a file.
// It supports category-based filtering, JSON formatting, and file rotation.
type FileSubscriber struct {
	id         string
	categories []event.EventCategory
	formatter  formatter.FormatterInterface
	adapter    adapter.OutputAdapterInterface
	logger     *log.Logger
}

var _ SubscriberInterface = (*FileSubscriber)(nil)

// init registers the file subscriber factory with the global registry.
// This runs before main() and only registers the factory function.
// No configuration access or instance creation happens here.
func init() {
	RegisterSubscriberFactory("file", func() SubscriberInterface {
		return NewFileSubscriber()
	})
}

// NewFileSubscriber creates a new file subscriber instance.
func NewFileSubscriber() *FileSubscriber {
	return &FileSubscriber{}
}

// IsEnabled checks if the file subscriber should be activated based on configuration.
func (fs *FileSubscriber) IsEnabled() bool {
	return config.GetServerRuntime().Config.Observability.Output.File.Enabled
}

// Initialize sets up the file subscriber with the provided configuration.
func (fs *FileSubscriber) Initialize() error {
	// Get config from observability service
	fileConfig := config.GetServerRuntime().Config.Observability.Output.File

	// Create formatter based on config using the Initialize pattern
	fmtr := formatter.Initialize(fileConfig.Format)

	// Determine file path
	filePath := fileConfig.FilePath
	if filePath == "" {
		observability := filepath.Join(config.GetServerRuntime().ServerHome, "logs", "observability")
		filePath = filepath.Join(observability, "observability.log")
	}

	if fs.adapter != nil {
		if err := fs.adapter.Close(); err != nil {
			fs.logger.Warn("failed to close existing file adapter", log.Error(err))
		}
	}
	// Create file adapter using the Initialize pattern
	adptr, err := adapter.InitializeFileAdapter(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file adapter for path %s: %w", filePath, err)
	}

	// Set categories
	fs.categories = convertCategories(fileConfig.Categories)
	if len(fs.categories) == 0 {
		fs.categories = []event.EventCategory{event.CategoryAll}
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return fmt.Errorf("failed to generate file subscriber ID: %w", err)
	}

	fs.id = id
	fs.formatter = fmtr
	fs.adapter = adptr
	fs.logger = log.GetLogger().With(log.String(log.LoggerKeyComponentName, fileSubscriberComponentName))

	fs.logger.Debug("File subscriber initialized",
		log.String("filePath", filePath),
		log.String("format", fileConfig.Format),
		log.Int("categories", len(fs.categories)))

	return nil
}

// GetID returns the unique identifier for this subscriber.
func (fs *FileSubscriber) GetID() string {
	return fs.id
}

// GetCategories returns the categories this subscriber is interested in.
func (fs *FileSubscriber) GetCategories() []event.EventCategory {
	if len(fs.categories) > 0 {
		return fs.categories
	}
	// Default: all categories
	return []event.EventCategory{event.CategoryAll}
}

// OnEvent is called when a new event is published.
func (fs *FileSubscriber) OnEvent(evt *event.Event) error {
	return processEvent(evt, fs.formatter, fs.adapter, fs.logger, "file")
}

// Close closes the subscriber and releases resources.
func (fs *FileSubscriber) Close() error {
	fs.logger.Info("Closing file subscriber", log.String("subscriberID", fs.id))

	// Flush and close adapter
	if fs.adapter != nil {
		if err := fs.adapter.Flush(); err != nil {
			fs.logger.Error("Failed to flush file adapter", log.Error(err))
		}

		if err := fs.adapter.Close(); err != nil {
			fs.logger.Error("Failed to close file adapter", log.Error(err))
			return err
		}
	}

	fs.logger.Info("File subscriber closed", log.String("subscriberID", fs.id))
	return nil
}
