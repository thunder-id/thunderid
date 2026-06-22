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

// Package dbtypes holds neutral data-source type identifiers shared across the
// database layer. It has no SQL or Redis dependencies so that both the SQL
// provider and the Redis provider (and their consumers) can reference these
// constants without importing one another.
package dbtypes

// DataSourceTypeRedis is the type identifier for a Redis data source.
const DataSourceTypeRedis = "redis"
