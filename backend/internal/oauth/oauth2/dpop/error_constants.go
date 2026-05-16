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

package dpop

import "errors"

// ErrInvalidProof indicates the supplied DPoP proof failed validation. All failure
// modes wrap into this single sentinel so callers can map to the invalid_dpop_proof
// error code without inspecting the underlying cause.
var ErrInvalidProof = errors.New("invalid DPoP proof")

// ErrReplayedProof indicates a DPoP proof's jti was already accepted within the window.
var ErrReplayedProof = errors.New("DPoP proof replayed")

// ErrJktMismatch indicates the proof's computed jkt does not match the expected jkt.
var ErrJktMismatch = errors.New("DPoP proof jkt does not match expected jkt")
