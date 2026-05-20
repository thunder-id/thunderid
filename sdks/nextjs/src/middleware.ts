/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

/**
 * Edge Runtime entry point — safe for use in Next.js middleware.ts.
 *
 * This file must only import modules whose full transitive dependency graph
 * contains zero Node.js-only APIs (process.versions, fs, crypto, etc.).
 * Permitted dependencies: jose, fetch, next/server, and local utilities
 * that themselves satisfy the same constraint.
 *
 * Do NOT import from:
 *   - ThunderIDNextClient (depends on @thunderid/node → @thunderid/javascript)
 *   - server/ThunderIDProvider (depends on @thunderid/node)
 *   - server/actions/* (depend on @thunderid/node)
 *   - client/* (depend on @thunderid/javascript via @thunderid/react)
 */

export {default as thunderIDMiddleware} from './server/middleware/thunderIDMiddleware';
export * from './server/middleware/thunderIDMiddleware';

export {default as createRouteMatcher} from './server/middleware/createRouteMatcher';
