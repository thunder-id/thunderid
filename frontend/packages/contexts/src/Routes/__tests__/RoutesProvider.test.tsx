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

import {act} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {beforeEach, afterEach, describe, expect, it} from 'vitest';
import RoutesProvider from '../RoutesProvider';
import useRoutes from '../useRoutes';

interface TestRoutePaths {
  widgets: {
    detail: (id: string) => string;
  };
}

function WidgetDetailLink({id}: {id: string}) {
  const routes = useRoutes<Partial<TestRoutePaths>>();
  const href = routes.widgets?.detail(id) ?? `/fallback-widgets/${id}`;
  return (
    <a data-testid="widget-link" href={href}>
      Widget
    </a>
  );
}

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
});

describe('RoutesProvider', () => {
  it('exposes the supplied paths to descendants via useRoutes', () => {
    const paths: TestRoutePaths = {
      widgets: {detail: (id) => `/widgets/${id}`},
    };

    act(() => {
      root.render(
        <RoutesProvider paths={paths}>
          <WidgetDetailLink id="42" />
        </RoutesProvider>,
      );
    });

    expect(container.querySelector('[data-testid="widget-link"]')?.getAttribute('href')).toBe('/widgets/42');
  });

  it('falls back to the caller-provided default when rendered without a provider', () => {
    act(() => {
      root.render(<WidgetDetailLink id="42" />);
    });

    expect(container.querySelector('[data-testid="widget-link"]')?.getAttribute('href')).toBe('/fallback-widgets/42');
  });
});
