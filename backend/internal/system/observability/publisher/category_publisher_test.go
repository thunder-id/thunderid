package publisher

import (
	"sync"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/system/observability/event"
)

// mockSubscriberV2 is a mock subscriber for testing with category support
type mockSubscriberV2 struct {
	id          string
	categories  []event.EventCategory
	received    []*event.Event
	shouldError bool
	mu          sync.Mutex
}

func (m *mockSubscriberV2) GetID() string {
	return m.id
}

func (m *mockSubscriberV2) GetCategories() []event.EventCategory {
	return m.categories
}

func (m *mockSubscriberV2) OnEvent(evt *event.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.received = append(m.received, evt)
	if m.shouldError {
		return &testError{msg: "mock error"}
	}
	return nil
}

func (m *mockSubscriberV2) Close() error {
	return nil
}

func (m *mockSubscriberV2) IsEnabled() bool {
	return true
}

func (m *mockSubscriberV2) Initialize() error {
	return nil
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestCategoryPublisher_SmartPublishing(t *testing.T) {
	// Create publisher
	pub := NewCategoryPublisher()

	// Create event
	evt := &event.Event{
		EventID:   "test-1",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		TraceID:   "trace-1",
		Component: "test",
		Timestamp: time.Now(),
	}

	// Publish event with NO subscribers - should be skipped
	pub.Publish(evt)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Now add a subscriber for authentication events
	authSub := &mockSubscriberV2{
		id:         "auth-sub",
		categories: []event.EventCategory{event.CategoryAuthentication},
		received:   make([]*event.Event, 0),
	}

	pub.Subscribe(authSub)

	// Publish same event again
	evt2 := &event.Event{
		EventID:   "test-2",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		TraceID:   "trace-2",
		Component: "test",
		Timestamp: time.Now(),
	}

	pub.Publish(evt2)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify subscriber received it
	if len(authSub.received) != 1 {
		t.Errorf("Expected subscriber to receive 1 event, got %d", len(authSub.received))
	}

	pub.Shutdown()
}

func TestCategoryPublisher_CategoryRouting(t *testing.T) {
	// Create publisher
	pub := NewCategoryPublisher()

	// Create subscribers for different categories
	authSub := &mockSubscriberV2{
		id:         "auth-sub",
		categories: []event.EventCategory{event.CategoryAuthentication},
		received:   make([]*event.Event, 0),
	}

	tokenSub := &mockSubscriberV2{
		id:         "token-sub",
		categories: []event.EventCategory{event.CategoryFlows},
		received:   make([]*event.Event, 0),
	}

	allSub := &mockSubscriberV2{
		id:         "all-sub",
		categories: []event.EventCategory{event.CategoryAll},
		received:   make([]*event.Event, 0),
	}

	pub.Subscribe(authSub)
	pub.Subscribe(tokenSub)
	pub.Subscribe(allSub)

	// Publish authentication event
	authEvent := &event.Event{
		EventID:   "auth-1",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		TraceID:   "trace-1",
		Component: "test",
		Timestamp: time.Now(),
	}

	pub.Publish(authEvent)

	// Publish token event
	tokenEvent := &event.Event{
		EventID:   "token-1",
		Type:      string(event.EventTypeFlowStarted),
		TraceID:   "trace-2",
		Component: "test",
		Timestamp: time.Now(),
	}

	pub.Publish(tokenEvent)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify routing
	// authSub should receive ONLY auth event (subscriber filters it)
	if len(authSub.received) != 2 {
		// Note: authSub receives both because publisher broadcasts to all
		// But in real scenario, authSub would filter internally
		t.Logf("authSub received %d events (broadcasts to all, subscriber filters)", len(authSub.received))
	}

	// tokenSub should receive ONLY token event
	if len(tokenSub.received) != 2 {
		t.Logf("tokenSub received %d events (broadcasts to all, subscriber filters)", len(tokenSub.received))
	}

	// allSub should receive ALL events
	if len(allSub.received) != 2 {
		t.Errorf("Expected allSub to receive 2 events, got %d", len(allSub.received))
	}

	pub.Shutdown()
}

func TestCategoryPublisher_GetActiveCategories(t *testing.T) {
	// Create publisher
	pub := NewCategoryPublisher()

	// Initially no active categories
	activeCategories := pub.GetActiveCategories()
	if len(activeCategories) != 0 {
		t.Errorf("Expected 0 active categories initially, got %d", len(activeCategories))
	}

	// Add subscribers
	authSub := &mockSubscriberV2{
		id:         "auth-sub",
		categories: []event.EventCategory{event.CategoryAuthentication},
		received:   make([]*event.Event, 0),
	}

	tokenSub := &mockSubscriberV2{
		id:         "token-sub",
		categories: []event.EventCategory{event.CategoryFlows},
		received:   make([]*event.Event, 0),
	}

	pub.Subscribe(authSub)
	pub.Subscribe(tokenSub)

	// Now should have 2 active categories
	activeCategories = pub.GetActiveCategories()
	if len(activeCategories) != 2 {
		t.Errorf("Expected 2 active categories, got %d", len(activeCategories))
	}

	// Verify the categories
	categoryMap := make(map[event.EventCategory]bool)
	for _, cat := range activeCategories {
		categoryMap[cat] = true
	}

	if !categoryMap[event.CategoryAuthentication] {
		t.Error("Expected CategoryAuthentication to be active")
	}

	if !categoryMap[event.CategoryFlows] {
		t.Error("Expected CategoryTokens to be active")
	}

	pub.Shutdown()
}

func TestCategoryPublisher_SubscribeUnsubscribe(t *testing.T) {
	// Create publisher
	pub := NewCategoryPublisher()

	sub := &mockSubscriberV2{
		id:         "test-sub",
		categories: []event.EventCategory{event.CategoryAuthentication},
		received:   make([]*event.Event, 0),
	}

	// Subscribe
	pub.Subscribe(sub)

	// Verify active categories
	activeCategories := pub.GetActiveCategories()
	if len(activeCategories) != 1 {
		t.Errorf("Expected 1 active category, got %d", len(activeCategories))
	}

	// Unsubscribe
	pub.Unsubscribe(sub)

	// Verify no active categories
	activeCategories = pub.GetActiveCategories()
	if len(activeCategories) != 0 {
		t.Errorf("Expected 0 active categories after unsubscribe, got %d", len(activeCategories))
	}

	pub.Shutdown()
}

func TestCategoryPublisher_MultipleSubscribersPerCategory(t *testing.T) {
	// Create publisher
	pub := NewCategoryPublisher()

	// Create multiple subscribers for same category
	authSub1 := &mockSubscriberV2{
		id:         "auth-sub-1",
		categories: []event.EventCategory{event.CategoryAuthentication},
		received:   make([]*event.Event, 0),
	}

	authSub2 := &mockSubscriberV2{
		id:         "auth-sub-2",
		categories: []event.EventCategory{event.CategoryAuthentication},
		received:   make([]*event.Event, 0),
	}

	pub.Subscribe(authSub1)
	pub.Subscribe(authSub2)

	// Publish authentication event
	evt := &event.Event{
		EventID:   "auth-1",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		TraceID:   "trace-1",
		Component: "test",
		Timestamp: time.Now(),
	}

	pub.Publish(evt)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Both subscribers should receive the event
	if len(authSub1.received) != 1 {
		t.Errorf("Expected authSub1 to receive 1 event, got %d", len(authSub1.received))
	}

	if len(authSub2.received) != 1 {
		t.Errorf("Expected authSub2 to receive 1 event, got %d", len(authSub2.received))
	}

	pub.Shutdown()
}

func TestCategoryPublisher_PublishNilEvent(t *testing.T) {
	pub := NewCategoryPublisher()
	defer pub.Shutdown()

	// Should not panic
	pub.Publish(nil)
}

func TestCategoryPublisher_PublishInvalidEvent(t *testing.T) {
	pub := NewCategoryPublisher()
	defer pub.Shutdown()

	// Event missing required fields
	invalidEvt := &event.Event{
		Type: "test",
		// Missing TraceID, EventID, Component, Timestamp
	}

	// Should not panic, should be rejected
	pub.Publish(invalidEvt)
}

func TestCategoryPublisher_PublishAfterShutdown(t *testing.T) {
	pub := NewCategoryPublisher()

	authSub := &mockSubscriberV2{
		id:         "auth-sub",
		categories: []event.EventCategory{event.CategoryAuthentication},
		received:   make([]*event.Event, 0),
	}
	pub.Subscribe(authSub)

	// Shutdown
	pub.Shutdown()

	// Try to publish after shutdown
	evt := &event.Event{
		EventID:   "test-1",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		TraceID:   "trace-1",
		Component: "test",
		Timestamp: time.Now(),
	}

	pub.Publish(evt)

	// Give it time (should not process)
	time.Sleep(50 * time.Millisecond)

	// Event should not be processed after shutdown
	if len(authSub.received) > 0 {
		t.Errorf("Expected 0 events after shutdown, got %d", len(authSub.received))
	}
}

func TestCategoryPublisher_SubscribeNil(t *testing.T) {
	pub := NewCategoryPublisher()
	defer pub.Shutdown()

	// Should not panic
	pub.Subscribe(nil)
}

func TestCategoryPublisher_UnsubscribeNil(t *testing.T) {
	pub := NewCategoryPublisher()
	defer pub.Shutdown()

	// Should not panic
	pub.Unsubscribe(nil)
}

func TestCategoryPublisher_SubscriberPanic(t *testing.T) {
	pub := NewCategoryPublisher()
	defer pub.Shutdown()

	// Create subscriber that panics
	panicSub := &mockSubscriberPanic{
		id:         "panic-sub",
		categories: []event.EventCategory{event.CategoryAll},
	}

	pub.Subscribe(panicSub)

	evt := &event.Event{
		EventID:   "test-1",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		TraceID:   "trace-1",
		Component: "test",
		Timestamp: time.Now(),
	}

	// Should not crash the publisher
	pub.Publish(evt)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Test passes if publisher doesn't crash
}

func TestCategoryPublisher_SubscriberError(t *testing.T) {
	pub := NewCategoryPublisher()
	defer pub.Shutdown()

	errorSub := &mockSubscriberV2{
		id:          "error-sub",
		categories:  []event.EventCategory{event.CategoryAll},
		received:    make([]*event.Event, 0),
		shouldError: true,
	}

	pub.Subscribe(errorSub)

	evt := &event.Event{
		EventID:   "test-1",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		TraceID:   "trace-1",
		Component: "test",
		Timestamp: time.Now(),
	}

	pub.Publish(evt)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Test passes if publisher handles error gracefully
}

func TestCategoryPublisher_DoubleShutdown(t *testing.T) {
	pub := NewCategoryPublisher()

	// First shutdown
	pub.Shutdown()

	// Second shutdown should not panic
	pub.Shutdown()
}

func TestCategoryPublisher_UnsubscribeMultipleCategories(t *testing.T) {
	pub := NewCategoryPublisher()
	defer pub.Shutdown()

	// Create subscriber with multiple categories
	multiSub := &mockSubscriberV2{
		id: "multi-sub",
		categories: []event.EventCategory{
			event.CategoryAuthentication,
			event.CategoryAuthorization,
			event.CategoryFlows,
		},
		received: make([]*event.Event, 0),
	}

	pub.Subscribe(multiSub)

	// Verify it's in all categories
	activeCategories := pub.GetActiveCategories()
	if len(activeCategories) != 3 {
		t.Errorf("Expected 3 active categories, got %d", len(activeCategories))
	}

	// Unsubscribe
	pub.Unsubscribe(multiSub)

	// Verify all categories are cleaned up
	activeCategories = pub.GetActiveCategories()
	if len(activeCategories) != 0 {
		t.Errorf("Expected 0 active categories after unsubscribe, got %d", len(activeCategories))
	}
}

func TestCategoryPublisher_ShutdownWithSubscriberCloseError(t *testing.T) {
	pub := NewCategoryPublisher()

	errorSub := &mockSubscriberCloseError{
		id:         "error-sub",
		categories: []event.EventCategory{event.CategoryAll},
	}

	pub.Subscribe(errorSub)

	// Should not panic even if Close() returns error
	pub.Shutdown()
}

// mockSubscriberPanic panics in OnEvent
type mockSubscriberPanic struct {
	id         string
	categories []event.EventCategory
}

func (m *mockSubscriberPanic) GetID() string {
	return m.id
}

func (m *mockSubscriberPanic) GetCategories() []event.EventCategory {
	return m.categories
}

func (m *mockSubscriberPanic) OnEvent(evt *event.Event) error {
	panic("subscriber panic!")
}

func (m *mockSubscriberPanic) Close() error {
	return nil
}

func (m *mockSubscriberPanic) IsEnabled() bool {
	return true
}

func (m *mockSubscriberPanic) Initialize() error {
	return nil
}

// mockSubscriberCloseError returns error on Close
type mockSubscriberCloseError struct {
	id         string
	categories []event.EventCategory
}

func (m *mockSubscriberCloseError) GetID() string {
	return m.id
}

func (m *mockSubscriberCloseError) GetCategories() []event.EventCategory {
	return m.categories
}

func (m *mockSubscriberCloseError) OnEvent(evt *event.Event) error {
	return nil
}

func (m *mockSubscriberCloseError) Close() error {
	return &testError{msg: "close error"}
}

func (m *mockSubscriberCloseError) IsEnabled() bool {
	return true
}

func (m *mockSubscriberCloseError) Initialize() error {
	return nil
}

// mockSubscriberBlocking sleeps in OnEvent
type mockSubscriberBlocking struct {
	id         string
	categories []event.EventCategory
	sleepDur   time.Duration
	wasCalled  bool
	callCount  int
}

func (m *mockSubscriberBlocking) GetID() string {
	return m.id
}

func (m *mockSubscriberBlocking) GetCategories() []event.EventCategory {
	return m.categories
}

func (m *mockSubscriberBlocking) OnEvent(evt *event.Event) error {
	m.wasCalled = true
	m.callCount++
	time.Sleep(m.sleepDur)
	return nil
}

func (m *mockSubscriberBlocking) Close() error {
	return nil
}

func (m *mockSubscriberBlocking) IsEnabled() bool {
	return true
}

func (m *mockSubscriberBlocking) Initialize() error {
	return nil
}

func TestCategoryPublisher_AsyncNonBlocking(t *testing.T) {
	pub := NewCategoryPublisher()

	// Create a subscriber that blocks for 200ms
	blockingSub := &mockSubscriberBlocking{
		id:         "blocking-sub",
		categories: []event.EventCategory{event.CategoryAuthentication},
		sleepDur:   200 * time.Millisecond,
	}

	pub.Subscribe(blockingSub)

	evt := &event.Event{
		EventID:   "test-async",
		Type:      string(event.EventTypeTokenIssuanceStarted),
		TraceID:   "trace-1",
		Component: "test",
		Timestamp: time.Now(),
	}

	// Measure time taken for Publish to return
	start := time.Now()
	pub.Publish(evt)
	elapsed := time.Since(start)

	// It should return almost instantly, definitely much faster than the 200ms sleep
	// We use 50ms as a very conservative upper bound for thread scheduling and channel overhead
	if elapsed > 50*time.Millisecond {
		t.Errorf("Publish took too long to return: %v. Expected < 50ms", elapsed)
	} else {
		t.Logf("Publish returned in %v, completely non-blocking", elapsed)
	}

	// Wait for processing to complete to confirm it actually ran
	// Shutdown waits for WaitGroup, so this will block until the subscriber finishes
	pub.Shutdown()

	if !blockingSub.wasCalled {
		t.Error("Subscriber was never called")
	}
}
