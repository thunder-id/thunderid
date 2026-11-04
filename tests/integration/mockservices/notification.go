// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mockservices

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

// NotificationServer provides a mock HTTP server for testing SMS notifications
type NotificationServer struct {
	handler  http.Handler
	baseURL  string
	messages []SMSMessage
	mutex    sync.RWMutex
	// failStatus, when 400 or above, is returned instead of a success response.
	failStatus int
}

// SMSMessage represents a received SMS message, along with the details of the request that carried
// it, so tests can assert how a sender dispatched the message and not just what it sent.
type SMSMessage struct {
	Message     string            `json:"message"`
	OTP         string            `json:"otp,omitempty"`
	Method      string            `json:"method,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	Query       string            `json:"query,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// SMSRequest represents the expected SMS request format
type SMSRequest struct {
	Body string `json:"body"`
}

// NewNotificationServer creates a new mock notification server
func NewNotificationServer(baseURL string) *NotificationServer {
	m := &NotificationServer{
		baseURL:  baseURL,
		messages: make([]SMSMessage, 0),
	}
	m.handler = m.routes()
	return m
}

// routes maps the mock's endpoints.
func (m *NotificationServer) routes() http.Handler {
	mux := http.NewServeMux()

	// Handle SMS sending endpoint
	mux.HandleFunc("/send-sms", m.handleSendSMS)

	// Handle message retrieval endpoint for testing
	mux.HandleFunc("/messages", m.handleGetMessages)

	// Handle clear messages endpoint for testing
	mux.HandleFunc("/clear", m.handleClearMessages)

	return mux
}

// GetURL returns the base URL of the mock server
func (m *NotificationServer) GetURL() string {
	return m.baseURL
}

// GetSendSMSURL returns the SMS sending endpoint URL
func (m *NotificationServer) GetSendSMSURL() string {
	return fmt.Sprintf("%s/send-sms", m.GetURL())
}

// SetResponseStatus makes subsequent send requests return the given status, so tests can exercise
// how a sender failure surfaces. A status below 400 restores the normal success response.
func (m *NotificationServer) SetResponseStatus(status int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.failStatus = status
}

// handleSendSMS handles SMS sending requests. Any method is accepted so that senders configured to
// use GET can be exercised; the method actually used is recorded on the message.
func (m *NotificationServer) handleSendSMS(w http.ResponseWriter, r *http.Request) {
	// Read the raw body as the SMS message content
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	messageBody := string(bodyBytes)

	// A sender using GET carries the message in the query string rather than the body.
	otpSource := messageBody
	if messageBody == "" {
		otpSource = r.URL.RawQuery
	}
	otp := extractOTPFromMessage(otpSource)

	headers := make(map[string]string, len(r.Header))
	for name := range r.Header {
		headers[name] = r.Header.Get(name)
	}

	message := SMSMessage{
		Message:     messageBody,
		OTP:         otp,
		Method:      r.Method,
		ContentType: r.Header.Get("Content-Type"),
		Query:       r.URL.RawQuery,
		Headers:     headers,
	}

	m.mutex.Lock()
	m.messages = append(m.messages, message)
	failStatus := m.failStatus
	messageCount := len(m.messages)
	m.mutex.Unlock()

	log.Printf("Mock SMS received via %s: %s (OTP: %s)", r.Method, messageBody, otp)

	if failStatus >= 400 {
		http.Error(w, "mock sender failure", failStatus)
		return
	}

	// Return success response
	response := map[string]interface{}{
		"success":   true,
		"messageId": fmt.Sprintf("mock-msg-%d", messageCount),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleGetMessages handles requests to retrieve sent messages
func (m *NotificationServer) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	messages := make([]SMSMessage, len(m.messages))
	copy(messages, m.messages)
	m.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(messages)
}

// handleClearMessages handles requests to clear all messages
func (m *NotificationServer) handleClearMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.mutex.Lock()
	m.messages = make([]SMSMessage, 0)
	m.mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

// GetLastMessage returns the last received message
func (m *NotificationServer) GetLastMessage() *SMSMessage {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(m.messages) == 0 {
		return nil
	}
	lastMessage := m.messages[len(m.messages)-1]
	m.messages = m.messages[:len(m.messages)-1]
	return &lastMessage
}

// GetMessages returns every message held, oldest first, without consuming them.
func (m *NotificationServer) GetMessages() []SMSMessage {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	messages := make([]SMSMessage, len(m.messages))
	copy(messages, m.messages)
	return messages
}

// ClearMessages clears all stored messages
func (m *NotificationServer) ClearMessages() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.messages = make([]SMSMessage, 0)
}

// extractOTPFromMessage extracts numeric OTP from the message text
// Handles formats like "Your verification code is: 657786. This code is valid for 2 minutes."
func extractOTPFromMessage(message string) string {
	var currentNumber string
	var bestSequence string
	var bestScore int

	for _, char := range message {
		if char >= '0' && char <= '9' {
			currentNumber += string(char)
		} else {
			// When we hit a non-digit, check if current sequence is a valid OTP length
			if len(currentNumber) >= 4 && len(currentNumber) <= 8 {
				score := calculateOTPScore(currentNumber)
				if score > bestScore {
					bestSequence = currentNumber
					bestScore = score
				}
			}
			currentNumber = ""
		}
	}

	// Check the last sequence too
	if len(currentNumber) >= 4 && len(currentNumber) <= 8 {
		score := calculateOTPScore(currentNumber)
		if score > bestScore {
			bestSequence = currentNumber
		}
	}

	return bestSequence
}

// calculateOTPScore assigns a score to potential OTP sequences
// Prioritizes 6-digit codes, then length, to find the most likely OTP
func calculateOTPScore(sequence string) int {
	length := len(sequence)

	// 6-digit codes are most common for SMS OTP
	if length == 6 {
		return 100
	}
	// 4-digit codes are second most common
	if length == 4 {
		return 80
	}
	// 5-digit codes
	if length == 5 {
		return 70
	}
	// 8-digit codes (less common but valid)
	if length == 8 {
		return 60
	}
	// 7-digit codes (least common)
	if length == 7 {
		return 50
	}

	return 0
}
