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

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/formatter"
)

// Format constants - duplicated here to avoid import cycle with observability package
const (
	formatJSON = "json"
	formatCSV  = "csv"
)

// writerAdapter defines the interface for writing formatted data.
type writerAdapter interface {
	Write(data []byte) error
}

// convertCategories converts string categories to EventCategory types.
// This is a local helper to avoid importing the observability package (which would create a cycle).
func convertCategories(stringCategories []string) []event.EventCategory {
	categories := make([]event.EventCategory, 0, len(stringCategories))
	for _, cat := range stringCategories {
		categories = append(categories, event.EventCategory(cat))
	}
	return categories
}

// processEvent is a shared helper that formats an event and writes it using the provided adapter.
// This eliminates duplicate code between ConsoleSubscriber and FileSubscriber OnEvent implementations.
func processEvent(
	evt *event.Event,
	fmtr formatter.FormatterInterface,
	adapter writerAdapter,
	logger *log.Logger,
	outputType string,
) error {
	if evt == nil {
		return fmt.Errorf("event is nil")
	}

	// Format the event
	formattedData, err := fmtr.Format(evt)
	if err != nil {
		logger.Error("Failed to format event",
			log.String("eventType", evt.Type),
			log.String("eventID", evt.EventID),
			log.Error(err))
		return fmt.Errorf("failed to format event: %w", err)
	}

	// Write using adapter
	if err := adapter.Write(formattedData); err != nil {
		logger.Error(fmt.Sprintf("Failed to write event to %s", outputType),
			log.String("eventType", evt.Type),
			log.String("eventID", evt.EventID),
			log.Error(err))
		return fmt.Errorf("failed to write to %s: %w", outputType, err)
	}

	logger.Debug(fmt.Sprintf("Event processed successfully to %s", outputType),
		log.String("eventType", evt.Type),
		log.String("eventID", evt.EventID),
		log.String("traceID", evt.TraceID))

	return nil
}
