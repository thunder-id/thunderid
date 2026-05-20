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

// Package jose provides JSON Object Signing and Encryption (JOSE) functionality.
// It includes support for JWS (JSON Web Signature), JWT (JSON Web Token), and JWE (JSON Web Encryption).
package jose

import (
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pkiservice"
)

// Initialize initializes the JOSE services (JWT and JWE).
func Initialize(pkiService pkiservice.PKIServiceInterface) (jwt.JWTServiceInterface, jwe.JWEServiceInterface, error) {
	jwtService, err := jwt.Initialize(pkiService)
	if err != nil {
		return nil, nil, err
	}

	jweService, err := jwe.Initialize(pkiService)
	if err != nil {
		return nil, nil, err
	}

	return jwtService, jweService, nil
}
