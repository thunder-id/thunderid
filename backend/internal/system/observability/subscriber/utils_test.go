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
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/observability/event"
)

// getCommonEventTestCases returns common test cases for event processing
// used by both console and file subscriber tests to avoid duplication.
func getCommonEventTestCases() []struct {
	name    string
	event   *event.Event
	wantErr bool
} {
	return []struct {
		name    string
		event   *event.Event
		wantErr bool
	}{
		{
			name:    "error when event is nil",
			event:   nil,
			wantErr: true,
		},
		{
			name: "successfully processes valid event",
			event: &event.Event{
				TraceID:   "trace-123",
				EventID:   "event-123",
				Type:      "test.event",
				Timestamp: time.Now(),
				Component: "TestComponent",
				Status:    event.StatusSuccess,
				Data: map[string]interface{}{
					"key1": "value1",
					"key2": 123,
				},
			},
			wantErr: false,
		},
		{
			name: "successfully processes event with complex data",
			event: &event.Event{
				TraceID:   "trace-456",
				EventID:   "event-456",
				Type:      "test.complex",
				Timestamp: time.Now(),
				Component: "TestComponent",
				Status:    event.StatusSuccess,
				Data: map[string]interface{}{
					"string": "value",
					"int":    42,
					"float":  3.14,
					"bool":   true,
					"nested": map[string]interface{}{"key": "value"},
					"array":  []string{"a", "b", "c"},
				},
			},
			wantErr: false,
		},
		{
			name: "successfully processes failure event",
			event: &event.Event{
				TraceID:   "trace-789",
				EventID:   "event-789",
				Type:      "test.failure",
				Timestamp: time.Now(),
				Component: "TestComponent",
				Status:    event.StatusFailure,
				Data: map[string]interface{}{
					event.DataKey.Error: "test error message",
				},
			},
			wantErr: false,
		},
	}
}

func TestConvertCategories(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantLen  int
		validate func(*testing.T, []event.EventCategory)
	}{
		{
			name:    "empty slice",
			input:   []string{},
			wantLen: 0,
			validate: func(t *testing.T, cats []event.EventCategory) {
				if len(cats) != 0 {
					t.Errorf("Expected empty slice, got %d categories", len(cats))
				}
			},
		},
		{
			name:    "nil slice",
			input:   nil,
			wantLen: 0,
			validate: func(t *testing.T, cats []event.EventCategory) {
				if cats == nil {
					t.Error("Result should not be nil, should be empty slice")
				}
			},
		},
		{
			name:    "single category",
			input:   []string{"observability.authentication"},
			wantLen: 1,
			validate: func(t *testing.T, cats []event.EventCategory) {
				if cats[0] != event.EventCategory("observability.authentication") {
					t.Errorf("Category = %s, want observability.authentication", cats[0])
				}
			},
		},
		{
			name: "multiple categories",
			input: []string{
				"observability.authentication",
				"observability.flows",
				"observability.authorization",
			},
			wantLen: 3,
			validate: func(t *testing.T, cats []event.EventCategory) {
				expectedCategories := map[event.EventCategory]bool{
					event.EventCategory("observability.authentication"): false,
					event.EventCategory("observability.flows"):          false,
					event.EventCategory("observability.authorization"):  false,
				}

				for _, cat := range cats {
					if _, exists := expectedCategories[cat]; exists {
						expectedCategories[cat] = true
					}
				}

				for cat, found := range expectedCategories {
					if !found {
						t.Errorf("Expected category %s not found", cat)
					}
				}
			},
		},
		{
			name:    "category with special characters",
			input:   []string{"observability.all"},
			wantLen: 1,
			validate: func(t *testing.T, cats []event.EventCategory) {
				if cats[0] != event.CategoryAll {
					t.Errorf("Category = %s, want %s", cats[0], event.CategoryAll)
				}
			},
		},
		{
			name: "duplicate categories",
			input: []string{
				"observability.authentication",
				"observability.authentication",
			},
			wantLen: 2, // Duplicates are not filtered
			validate: func(t *testing.T, cats []event.EventCategory) {
				if len(cats) != 2 {
					t.Errorf("Expected 2 categories (duplicates not filtered), got %d", len(cats))
				}
			},
		},
		{
			name: "empty string in slice",
			input: []string{
				"observability.authentication",
				"",
				"observability.flows",
			},
			wantLen: 3, // Empty strings are not filtered
			validate: func(t *testing.T, cats []event.EventCategory) {
				if len(cats) != 3 {
					t.Errorf("Expected 3 categories, got %d", len(cats))
				}
				// Check that empty string was converted
				hasEmpty := false
				for _, cat := range cats {
					if string(cat) == "" {
						hasEmpty = true
						break
					}
				}
				if !hasEmpty {
					t.Error("Expected empty category to be present")
				}
			},
		},
		{
			name: "case sensitivity preserved",
			input: []string{
				"observability.Authentication",
				"observability.authentication",
			},
			wantLen: 2,
			validate: func(t *testing.T, cats []event.EventCategory) {
				if string(cats[0]) == string(cats[1]) {
					t.Error("Case should be preserved, categories should be different")
				}
			},
		},
		{
			name: "whitespace preserved",
			input: []string{
				"observability.authentication ",
				" observability.authentication",
			},
			wantLen: 2,
			validate: func(t *testing.T, cats []event.EventCategory) {
				if string(cats[0]) == string(cats[1]) {
					t.Error("Whitespace should be preserved, categories should be different")
				}
			},
		},
		{
			name: "all standard categories",
			input: []string{
				"observability.authentication",
				"observability.authorization",
				"observability.flows",
				"observability.all",
			},
			wantLen: 4,
			validate: func(t *testing.T, cats []event.EventCategory) {
				expectedCount := 4
				if len(cats) != expectedCount {
					t.Errorf("Expected %d categories, got %d", expectedCount, len(cats))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertCategories(tt.input)

			if len(result) != tt.wantLen {
				t.Errorf("convertCategories() returned %d categories, want %d", len(result), tt.wantLen)
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestConvertCategories_TypeConversion(t *testing.T) {
	input := []string{"test.category"}
	result := convertCategories(input)

	// Verify the result is of type []event.EventCategory
	var _ []event.EventCategory = result

	// Verify we can use it as EventCategory
	if len(result) != 1 {
		t.Fatalf("Expected 1 category, got %d", len(result))
	}

	category := result[0]
	if string(category) != "test.category" {
		t.Errorf("Category value = %s, want test.category", category)
	}
}

func TestConvertCategories_PreservesOrder(t *testing.T) {
	input := []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
	}

	result := convertCategories(input)

	if len(result) != len(input) {
		t.Fatalf("Length mismatch: got %d, want %d", len(result), len(input))
	}

	for i, expected := range input {
		if string(result[i]) != expected {
			t.Errorf("Order not preserved at index %d: got %s, want %s", i, result[i], expected)
		}
	}
}

func TestConvertCategories_LargeSlice(t *testing.T) {
	// Test with a large number of categories
	size := 1000
	input := make([]string, size)
	for i := 0; i < size; i++ {
		input[i] = "category_" + string(rune(i))
	}

	result := convertCategories(input)

	if len(result) != size {
		t.Errorf("Expected %d categories, got %d", size, len(result))
	}
}

func TestConvertCategories_Capacity(t *testing.T) {
	input := []string{
		"observability.authentication",
		"observability.flows",
	}

	result := convertCategories(input)

	// Check that the capacity was pre-allocated correctly
	if cap(result) < len(input) {
		t.Errorf("Capacity = %d, should be at least %d", cap(result), len(input))
	}
}

func TestConvertCategories_NoMutation(t *testing.T) {
	input := []string{
		"observability.authentication",
		"observability.flows",
	}

	// Store original values
	originalFirst := input[0]
	originalSecond := input[1]

	// Convert
	_ = convertCategories(input)

	// Verify input was not mutated
	if input[0] != originalFirst {
		t.Error("Input slice was mutated (first element)")
	}
	if input[1] != originalSecond {
		t.Error("Input slice was mutated (second element)")
	}
}

func TestConvertCategories_IndependentSlices(t *testing.T) {
	input := []string{"observability.authentication"}
	result1 := convertCategories(input)
	result2 := convertCategories(input)

	// Modify result1
	result1[0] = event.EventCategory("modified")

	// result2 should not be affected
	if result2[0] == "modified" {
		t.Error("Modifying one result affected another result")
	}
}

func TestFormatConstants(t *testing.T) {
	// Test that format constants match expected values
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "json format",
			constant: formatJSON,
			expected: "json",
		},
		{
			name:     "csv format",
			constant: formatCSV,
			expected: "csv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Format constant = %s, want %s", tt.constant, tt.expected)
			}
		})
	}
}

func TestFormatConstants_AreStrings(t *testing.T) {
	// Verify format constants are strings
	var _ string = formatJSON
	var _ string = formatCSV
}

func TestFormatConstants_NotEmpty(t *testing.T) {
	if formatJSON == "" {
		t.Error("formatJSON should not be empty")
	}
	if formatCSV == "" {
		t.Error("formatCSV should not be empty")
	}
}

func TestFormatConstants_Unique(t *testing.T) {
	if formatJSON == formatCSV {
		t.Error("Format constants should be unique")
	}
}

func BenchmarkConvertCategories_Small(b *testing.B) {
	input := []string{
		"observability.authentication",
		"observability.flows",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertCategories(input)
	}
}

func BenchmarkConvertCategories_Medium(b *testing.B) {
	input := make([]string, 10)
	for i := 0; i < 10; i++ {
		input[i] = "category_" + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertCategories(input)
	}
}

func BenchmarkConvertCategories_Large(b *testing.B) {
	input := make([]string, 100)
	for i := 0; i < 100; i++ {
		input[i] = "category_" + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertCategories(input)
	}
}

func BenchmarkConvertCategories_Empty(b *testing.B) {
	input := []string{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertCategories(input)
	}
}
