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

// Package model holds public data types for the inbound client subsystem.
//
//nolint:lll
package model

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

type (
	// InboundClient is the persistence shape for protocol-agnostic inbound client record.
	InboundClient = providers.InboundClient
	// AssertionConfig is the entity-level assertion config; token configs fall back to it.
	AssertionConfig = providers.AssertionConfig
	// LoginConsentConfig is the login consent configuration.
	LoginConsentConfig = providers.LoginConsentConfig
	// Certificate is a user-supplied certificate input.
	Certificate = providers.Certificate
)

// InboundClientAttributes is the flattened view of one inbound client's configured user attributes.
type InboundClientAttributes struct {
	InboundClientID string
	Attributes      []string
}

// DeclarativeLoaderConfig describes how to load inbound clients from a YAML resource directory.
type DeclarativeLoaderConfig struct {
	ResourceType  string
	DirectoryName string
	Parser        func(data []byte) (*InboundClient, error)
	Validator     func(*InboundClient) error
}
