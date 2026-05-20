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

// Package jwt provides functionalities for handling JSON Web Tokens (JWTs).
package jwt

import (
	"time"

	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pkiservice"
)

// Initialize initializes the JWT service.
func Initialize(pkiSvc pkiservice.PKIServiceInterface) (JWTServiceInterface, error) {
	httpClient := httpservice.NewHTTPClientWithTimeout(10 * time.Second)
	runtimeSvc := defaultkm.NewRuntimeCryptoService(pkiSvc, nil)
	return newJWTService(pkiSvc, httpClient, runtimeSvc)
}
