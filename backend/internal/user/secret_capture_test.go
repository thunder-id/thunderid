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

package user

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

type recordingCapturer struct {
	calls [][3]string
}

func (r *recordingCapturer) CaptureSecret(_ context.Context, _, resourceName, field, value string) {
	r.calls = append(r.calls, [3]string{resourceName, field, value})
}

func TestCaptureUserCredentials(t *testing.T) {
	t.Run("captures each schema credential under <username>_<credential>", func(t *testing.T) {
		etMock := entitytypemock.NewEntityTypeServiceInterfaceMock(t)
		etMock.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "customer",
			entitytype.AttributeFilter{AllowCredential: true}).
			Return([]entitytype.AttributeInfo{{Attribute: "password"}}, nil)

		rec := &recordingCapturer{}
		us := &userService{entityTypeService: etMock, secretCapturer: rec}

		us.captureUserCredentials(context.Background(), &User{
			Type:       "customer",
			Attributes: json.RawMessage(`{"username":"alice","password":"pw123","email":"a@b.co"}`),
		})

		assert.Equal(t, [][3]string{{"alice", "password", "pw123"}}, rec.calls)
	})

	t.Run("no-op when no capturer is configured", func(t *testing.T) {
		us := &userService{}
		assert.NotPanics(t, func() {
			us.captureUserCredentials(context.Background(), &User{
				Type:       "customer",
				Attributes: json.RawMessage(`{"username":"alice","password":"pw123"}`),
			})
		})
	})

	t.Run("skips when username is absent", func(t *testing.T) {
		rec := &recordingCapturer{}
		us := &userService{entityTypeService: entitytypemock.NewEntityTypeServiceInterfaceMock(t), secretCapturer: rec}

		us.captureUserCredentials(context.Background(), &User{
			Type:       "customer",
			Attributes: json.RawMessage(`{"password":"pw123"}`),
		})

		assert.Empty(t, rec.calls)
	})
}
