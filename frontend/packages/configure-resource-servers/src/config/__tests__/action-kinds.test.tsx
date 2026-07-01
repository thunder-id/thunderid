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

import {render} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import {getActionKindIcon} from '../get-action-kind-icon';
import {getActionKindLabel} from '../get-action-kind-label';
import {getNamespaceIcon} from '../get-namespace-icon';

const t = (key: string, fallback: string): string => fallback;

describe('getActionKindLabel', () => {
  it('should return "Tool" for kind tool', () => {
    expect(getActionKindLabel('tool', t)).toBe('Tool');
  });

  it('should return "Resource" for kind resource', () => {
    expect(getActionKindLabel('resource', t)).toBe('Resource');
  });

  it('should return "Namespace" for undefined kind', () => {
    expect(getActionKindLabel(undefined, t)).toBe('Namespace');
  });
});

describe('getActionKindIcon', () => {
  it('should render an svg icon for kind tool', () => {
    const {container} = render(getActionKindIcon('tool'));
    expect(container.querySelector('svg')).not.toBeNull();
  });

  it('should render an svg icon for kind resource', () => {
    const {container} = render(getActionKindIcon('resource'));
    expect(container.querySelector('svg')).not.toBeNull();
  });

  it('should render an svg icon for undefined kind (namespace)', () => {
    const {container} = render(getActionKindIcon(undefined));
    expect(container.querySelector('svg')).not.toBeNull();
  });

  it('should render an svg icon when a custom size is provided', () => {
    const {container} = render(getActionKindIcon('tool', 24));
    expect(container.querySelector('svg')).not.toBeNull();
  });
});

describe('getNamespaceIcon', () => {
  it('should render a FolderOpen icon when expanded is true', () => {
    const {container} = render(getNamespaceIcon(true));
    expect(container.querySelector('svg')).not.toBeNull();
  });

  it('should render a Folder icon when expanded is false', () => {
    const {container} = render(getNamespaceIcon(false));
    expect(container.querySelector('svg')).not.toBeNull();
  });

  it('should render with a custom size', () => {
    const {container} = render(getNamespaceIcon(false, 24));
    expect(container.querySelector('svg')).not.toBeNull();
  });
});
