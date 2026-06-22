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

package entity

import (
	"testing"
)

type TestNotification struct {
	Sender string `json:"sender"`
	Type   string `json:"type"`
	Auth   string `json:"auth"`
}

func TestStoreBasicOperations(t *testing.T) {
	store := NewStore()

	// Test storing and retrieving with KeyType
	notification := TestNotification{Sender: "Notification Sender", Type: "SMS", Auth: "None"}
	key := NewCompositeKey("user123", KeyTypeNotification)

	// Test Set
	err := store.Set(key, notification)
	if err != nil {
		t.Errorf("Failed to set notification: %v", err)
	}

	// Test Get
	entity, err := store.Get(key)
	if err != nil {
		t.Fatalf("Failed to get notification: %v", err)
	}
	if entity == nil {
		t.Fatal("Expected entity, got nil")
		return
	}

	retrievedNotification, ok := entity.Data.(TestNotification)
	if !ok {
		t.Fatalf("Failed to type assert entity data to TestNotification")
	}
	if retrievedNotification.Sender != notification.Sender {
		t.Errorf("Expected sender %s, got %s", notification.Sender, retrievedNotification.Sender)
	}

	// Test CountByType
	countByType, err := store.CountByType(KeyTypeNotification)
	if err != nil {
		t.Errorf("Failed to count by type: %v", err)
	}
	if countByType != 1 {
		t.Errorf("Expected count by type 1, got %d", countByType)
	}

	// Test ListByType
	entities, err := store.ListByType(KeyTypeNotification)
	if err != nil {
		t.Errorf("Failed to list by type: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(entities))
	}

	// Test Delete
	err = store.Delete(key)
	if err != nil {
		t.Errorf("Failed to delete user: %v", err)
	}

	// Verify deletion
	_, err = store.Get(key)
	if err == nil {
		t.Error("Expected error when getting deleted entity, got nil")
	}
}

func TestStoreStringConvenienceMethods(t *testing.T) {
	store := NewStore()

	notification := TestNotification{Sender: "Notification Sender", Type: "SMS", Auth: "None"}

	// Test SetByString
	err := store.Set(NewCompositeKeyFromString("user456", "notification"), notification)
	if err != nil {
		t.Errorf("Failed to set notification by string: %v", err)
	}

	// Test GetByString
	entity, err := store.Get(NewCompositeKeyFromString("user456", "notification"))
	if err != nil {
		t.Fatalf("Failed to get notification by string: %v", err)
	}
	if entity == nil {
		t.Fatal("Expected entity, got nil")
		return
	}

	// Type assert the data back to TestNotification
	retrievedNotification, ok := entity.Data.(TestNotification)
	if !ok {
		t.Fatalf("Failed to type assert entity data to TestNotification")
	}
	if retrievedNotification.Sender != notification.Sender {
		t.Errorf("Expected sender %s, got %s", notification.Sender, retrievedNotification.Sender)
	}

	// Test DeleteByString
	err = store.Delete(NewCompositeKeyFromString("user456", "notification"))
	if err != nil {
		t.Errorf("Failed to delete notification by string: %v", err)
	}
}

func TestListByType_SortsByEntityID(t *testing.T) {
	store := NewStore()

	err := store.Set(NewCompositeKey("id-2", KeyTypeNotification), TestNotification{Sender: "s2"})
	if err != nil {
		t.Fatalf("Failed to set id-2 entity: %v", err)
	}

	err = store.Set(NewCompositeKey("id-1", KeyTypeNotification), TestNotification{Sender: "s1"})
	if err != nil {
		t.Fatalf("Failed to set id-1 entity: %v", err)
	}

	err = store.Set(NewCompositeKey("id-3", KeyTypeApplication), map[string]string{"name": "app"})
	if err != nil {
		t.Fatalf("Failed to set non-notification entity: %v", err)
	}

	entities, err := store.ListByType(KeyTypeNotification)
	if err != nil {
		t.Fatalf("Failed to list by type: %v", err)
	}

	if len(entities) != 2 {
		t.Fatalf("Expected 2 entities, got %d", len(entities))
	}

	if entities[0].ID.ID != "id-1" {
		t.Fatalf("Expected first entity ID id-1, got %s", entities[0].ID.ID)
	}

	if entities[1].ID.ID != "id-2" {
		t.Fatalf("Expected second entity ID id-2, got %s", entities[1].ID.ID)
	}
}

func TestKeyTypeValidation(t *testing.T) {
	// Test valid KeyTypes
	validTypes := []KeyType{
		KeyTypeApplication, KeyTypeNotification,
	}

	for _, kt := range validTypes {
		if !kt.IsValid() {
			t.Errorf("KeyType %s should be valid", kt)
		}
	}

	// Test invalid KeyType
	invalidType := KeyType("invalid")
	if invalidType.IsValid() {
		t.Error("Invalid KeyType should not be valid")
	}
}

func TestCompositeKeyOperations(t *testing.T) {
	key := NewCompositeKey("test123", KeyTypeNotification)

	if key.ID != "test123" {
		t.Errorf("Expected ID test123, got %s", key.ID)
	}
	if key.Type != KeyTypeNotification {
		t.Errorf("Expected Type %s, got %s", KeyTypeNotification, key.Type)
	}

	// Test String representation
	expected := "notification:test123"
	if key.String() != expected {
		t.Errorf("Expected string %s, got %s", expected, key.String())
	}

	// Test NewCompositeKeyFromString
	stringKey := NewCompositeKeyFromString("test456", "application")
	if stringKey.ID != "test456" {
		t.Errorf("Expected ID test456, got %s", stringKey.ID)
	}
	if stringKey.Type != KeyType("application") {
		t.Errorf("Expected Type application, got %s", stringKey.Type)
	}
}

func TestSingletonStore(t *testing.T) {
	// Get two instances and verify they are the same
	store1 := GetInstance()
	store2 := GetInstance()

	if store1 != store2 {
		t.Error("GetInstance should return the same instance (singleton pattern)")
	}

	// Clear the store first
	err := store1.Clear()
	if err != nil {
		t.Errorf("Failed to clear store1: %v", err)
	}

	// Add data through first instance
	notification := TestNotification{Sender: "Notification Sender", Type: "SMS", Auth: "None"}
	key := NewCompositeKey("singleton1", KeyTypeNotification)

	err = store1.Set(key, notification)
	if err != nil {
		t.Errorf("Failed to set notification in store1: %v", err)
	}

	// Verify data is accessible through second instance (same singleton)
	entity, err := store2.Get(key)
	if err != nil {
		t.Fatalf("Failed to get notification from store2: %v", err)
	}
	if entity == nil {
		t.Fatal("Expected entity, got nil")
		return
	}

	retrievedNotification, ok := entity.Data.(TestNotification)
	if !ok {
		t.Fatalf("Failed to type assert entity data to TestNotification")
	}
	if retrievedNotification.Sender != notification.Sender {
		t.Errorf("Expected sender %s, got %s", notification.Sender, retrievedNotification.Sender)
	}
}
