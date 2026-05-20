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

package testutils

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
)

// MockSMTPServer is a minimal TCP SMTP server for capturing outbound emails in tests.
// It speaks enough of the SMTP protocol to accept emails from Go's net/smtp client
// without TLS or authentication.
type MockSMTPServer struct {
	listener net.Listener
	emails   []EmailMessage
	mutex    sync.RWMutex
	port     int
	done     chan struct{}
}

// EmailMessage holds the captured content of a single SMTP email.
type EmailMessage struct {
	From string
	To   []string
	Body string
}

// NewMockSMTPServer creates a new MockSMTPServer listening on the given port.
func NewMockSMTPServer(port int) *MockSMTPServer {
	return &MockSMTPServer{
		port:   port,
		emails: make([]EmailMessage, 0),
		done:   make(chan struct{}),
	}
}

// Start begins accepting SMTP connections in a background goroutine.
func (m *MockSMTPServer) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", m.port))
	if err != nil {
		return fmt.Errorf("failed to start mock SMTP server on port %d: %w", m.port, err)
	}
	m.listener = ln

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-m.done:
					return
				default:
					log.Printf("Mock SMTP server accept error: %v", err)
					return
				}
			}
			go m.handleConnection(conn)
		}
	}()

	log.Printf("Mock SMTP server started on port %d", m.port)
	return nil
}

// Stop shuts down the SMTP server.
func (m *MockSMTPServer) Stop() error {
	select {
	case <-m.done:
		// already closed
	default:
		close(m.done)
	}
	if m.listener != nil {
		return m.listener.Close()
	}
	return nil
}

// GetPort returns the port the server is listening on.
func (m *MockSMTPServer) GetPort() int {
	return m.port
}

// ClearEmails removes all captured emails.
func (m *MockSMTPServer) ClearEmails() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.emails = make([]EmailMessage, 0)
}

// GetLastEmail returns and removes the most recently captured email, or nil if none.
func (m *MockSMTPServer) GetLastEmail() *EmailMessage {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if len(m.emails) == 0 {
		return nil
	}
	last := m.emails[len(m.emails)-1]
	m.emails = m.emails[:len(m.emails)-1]
	return &last
}

// handleConnection processes a single SMTP client connection.
func (m *MockSMTPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)

	writeLine := func(s string) {
		fmt.Fprintf(w, "%s\r\n", s)
		w.Flush()
	}

	// SMTP greeting
	writeLine("220 localhost Mock SMTP Server ready")

	var email EmailMessage
	var inData bool
	var bodyBuilder strings.Builder

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			// SMTP dot-stuffing: a lone "." ends the message
			if line == "." {
				inData = false
				email.Body = bodyBuilder.String()
				m.mutex.Lock()
				m.emails = append(m.emails, email)
				m.mutex.Unlock()
				log.Printf("Mock SMTP: captured email to %v", email.To)
				email = EmailMessage{}
				bodyBuilder.Reset()
				writeLine("250 OK")
			} else {
				// Un-stuff leading dots
				if strings.HasPrefix(line, "..") {
					line = line[1:]
				}
				bodyBuilder.WriteString(line + "\n")
			}
			continue
		}

		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			writeLine("250-localhost")
			writeLine("250 OK")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			email.From = extractSMTPAngle(line[10:])
			writeLine("250 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			email.To = append(email.To, extractSMTPAngle(line[8:]))
			writeLine("250 OK")
		case upper == "DATA":
			inData = true
			writeLine("354 End data with <CR><LF>.<CR><LF>")
		case upper == "QUIT":
			writeLine("221 Bye")
			return
		case strings.HasPrefix(upper, "RSET"):
			email = EmailMessage{}
			bodyBuilder.Reset()
			writeLine("250 OK")
		default:
			writeLine("250 OK")
		}
	}
}

// extractSMTPAngle strips angle brackets and whitespace from an SMTP address field.
func extractSMTPAngle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	return strings.TrimSpace(s)
}

// ExtractRecoveryLink searches the email body for a URL containing "inviteToken"
// and returns the first match. Returns "" if none is found.
func (e *EmailMessage) ExtractRecoveryLink() string {
	for _, word := range strings.Fields(e.Body) {
		// Strip surrounding HTML punctuation that may appear in href="..." values
		word = strings.Trim(word, `"'<>`)
		if strings.HasPrefix(word, "http") && strings.Contains(word, "inviteToken") {
			return word
		}
	}
	// Fallback: scan character-by-character for href values
	body := e.Body
	for {
		idx := strings.Index(body, "inviteToken")
		if idx == -1 {
			break
		}
		// Walk back to find the start of the URL
		start := idx
		for start > 0 && body[start-1] != '"' && body[start-1] != '\'' && body[start-1] != ' ' {
			start--
		}
		// Walk forward to find the end of the URL
		end := idx
		for end < len(body) && body[end] != '"' && body[end] != '\'' && body[end] != ' ' && body[end] != '\n' {
			end++
		}
		candidate := body[start:end]
		if strings.HasPrefix(candidate, "http") {
			return candidate
		}
		body = body[idx+len("inviteToken"):]
	}
	return ""
}

// ExtractQueryParam extracts a query parameter value from a raw URL string.
func ExtractQueryParam(rawURL, param string) string {
	// Try proper URL parsing first
	u, err := url.Parse(rawURL)
	if err == nil {
		if v := u.Query().Get(param); v != "" {
			return v
		}
	}
	// Fallback: string search
	key := param + "="
	idx := strings.Index(rawURL, key)
	if idx == -1 {
		return ""
	}
	rest := rawURL[idx+len(key):]
	end := strings.IndexAny(rest, "&\"'<> \t\n\r")
	if end == -1 {
		return rest
	}
	return rest[:end]
}
