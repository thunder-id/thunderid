/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

const OrganizationUnitTreeConstants = {
  PLACEHOLDER_SUFFIX: '__placeholder',
  EMPTY_SUFFIX: '__empty',
  ERROR_SUFFIX: '__error',
  ADD_CHILD_SUFFIX: '__addChild',
  LOAD_MORE_SUFFIX: '__loadMore',
  ROOT_PARENT_ID: '__root',
  ROOT_LOAD_MORE_ID: '__root__loadMore',
  PAGE_SIZE: 30,
  DEFAULT_AVATAR: 'avatar:shape=rounded,variant=anonymous_entity,content=pavilion,colors=0',
} as const;

export default OrganizationUnitTreeConstants;
