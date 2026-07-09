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

// Package runtimekm provides the default key manager implementation backed by PKI key material.
package runtimekm

import (
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/configkm"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/runtimekm/pki"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize returns a fully wired RuntimeCryptoProvider and ConfigCryptoProvider.
func Initialize(pkiSvc pki.PKIServiceInterface) (
	providers.RuntimeCryptoProvider, kmprovider.ConfigCryptoProvider, error,
) {
	cfgSvc, err := configkm.InitConfigProvider()
	if err != nil {
		return nil, nil, err
	}

	runtimeSvc := NewRuntimeCryptoService(pkiSvc, cfgSvc)
	return runtimeSvc, cfgSvc, nil
}
