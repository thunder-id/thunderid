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

package core

// segmentBoundary holds the parameters of a segment boundary, which is identified by a display-only prompt node.
// It contains the ID of the display-only prompt node that serves as the boundary, and the ID of the next node
// which is the start node of the next segment.
type segmentBoundary struct {
	boundaryNodeID string
	nextNodeID     string
}
