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

package common

import (
	"errors"
	"strings"
)

// Validate cleans and validates the email data payload.
func (e *EmailData) Validate() error {
	trimSlice := func(s []string) []string {
		if s == nil {
			return nil
		}
		res := make([]string, len(s))
		for i, v := range s {
			res[i] = strings.TrimSpace(v)
		}
		return res
	}

	e.Subject = strings.TrimSpace(e.Subject)
	e.To = trimSlice(e.To)
	e.CC = trimSlice(e.CC)
	e.BCC = trimSlice(e.BCC)

	if len(e.To) == 0 || len(e.To[0]) == 0 {
		return errors.New("recipient address cannot be empty")
	}

	for _, addressList := range [][]string{e.To, e.CC, e.BCC} {
		for _, address := range addressList {
			if strings.ContainsAny(address, CRLF) {
				return errors.New("recipient address contains invalid characters")
			}
		}
	}

	if strings.ContainsAny(e.Subject, CRLF) {
		return errors.New("subject contains invalid characters")
	}

	return nil
}

// Valid checks if the notification sender type is valid.
func (t NotificationSenderType) Valid() bool {
	switch t {
	case NotificationSenderTypeMessage,
		NotificationSenderTypeEmail:
		return true
	}
	return false
}

// Valid checks if the notification provider type is valid.
func (p NotificationProviderType) Valid() bool {
	switch p {
	case NotificationProviderTypeVonage,
		NotificationProviderTypeTwilio,
		NotificationProviderTypeCustom,
		NotificationProviderTypeSMTP,
		NotificationProviderTypeHTTP:
		return true
	}
	return false
}
