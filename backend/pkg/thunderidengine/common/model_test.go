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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestI18nMessage_String(t *testing.T) {
	msg := I18nMessage{
		Key:          "key",
		DefaultValue: "default value",
	}
	assert.Equal(t, "default value", msg.String())

	msgEmpty := I18nMessage{}
	assert.Equal(t, "", msgEmpty.String())
}

func TestI18nMessage_IsEmpty(t *testing.T) {
	t.Run("Empty Message", func(t *testing.T) {
		msg := I18nMessage{}
		assert.True(t, msg.IsEmpty())
	})

	t.Run("Non-Empty Message", func(t *testing.T) {
		msg := I18nMessage{Key: "key"}
		assert.False(t, msg.IsEmpty())
	})

	t.Run("Message with value only (invalid state technically but testing IsEmpty logic)", func(t *testing.T) {
		msg := I18nMessage{DefaultValue: "val"}
		assert.True(t, msg.IsEmpty())
	})
}
