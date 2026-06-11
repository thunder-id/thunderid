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

/* eslint-disable react-refresh/only-export-components */
import {Globe, MCP, PuzzleIcon} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import type {ResourceServerType} from '../models/resource-server';

export interface ResourceServerTypeMetadata {
  value: ResourceServerType;
  icon: JSX.Element;
  titleKey: string;
  titleFallback: string;
  descriptionKey: string;
}

const ResourceServerTypeMetadataList: ResourceServerTypeMetadata[] = [
  {
    value: 'API',
    icon: <Globe size={32} />,
    titleKey: 'resourceServers:create.type.api.title',
    titleFallback: 'API',
    descriptionKey: 'resourceServers:create.type.api.description',
  },
  {
    value: 'MCP',
    icon: <MCP size={32} />,
    titleKey: 'resourceServers:create.type.mcp.title',
    titleFallback: 'MCP',
    descriptionKey: 'resourceServers:create.type.mcp.description',
  },
  {
    value: 'CUSTOM',
    icon: <PuzzleIcon size={32} />,
    titleKey: 'resourceServers:create.type.custom.title',
    titleFallback: 'Custom',
    descriptionKey: 'resourceServers:create.type.custom.description',
  },
];

const CUSTOM_METADATA = ResourceServerTypeMetadataList.find((m) => m.value === 'CUSTOM')!;

export function getResourceServerTypeMetadata(type: ResourceServerType | undefined): ResourceServerTypeMetadata {
  return ResourceServerTypeMetadataList.find((m) => m.value === type) ?? CUSTOM_METADATA;
}

export function getResourceServerTypeIcon(type: ResourceServerType | undefined): JSX.Element {
  return getResourceServerTypeMetadata(type).icon;
}

export function getResourceServerTypeLabel(
  type: ResourceServerType | undefined,
  t: (key: string, fallback: string) => string,
): string {
  const meta = getResourceServerTypeMetadata(type);
  return t(meta.titleKey, meta.titleFallback);
}

export default ResourceServerTypeMetadataList;
