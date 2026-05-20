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

import {render, waitFor} from '@testing-library/react';
import {describe, it, expect, afterEach} from 'vitest';
import Helmet from '../Helmet';

describe('Helmet', () => {
  const originalTitle = document.title;

  afterEach(() => {
    document.title = originalTitle;
  });

  it('renders null (nothing in the component tree)', () => {
    const {container} = render(<Helmet />);
    expect(container.firstChild).toBeNull();
  });

  it('sets document.title from <title> child', async () => {
    render(
      <Helmet>
        <title>Test Title</title>
      </Helmet>,
    );
    await waitFor(() => expect(document.title).toBe('Test Title'));
  });

  it('appends <meta> tag to document.head', async () => {
    render(
      <Helmet>
        <meta name="description" content="Test description" />
      </Helmet>,
    );
    await waitFor(() => {
      const meta = document.querySelector('meta[name="description"]');
      expect(meta).toBeTruthy();
      expect(meta?.getAttribute('content')).toBe('Test description');
    });
  });

  it('appends <link> tag to document.head', async () => {
    render(
      <Helmet>
        <link rel="icon" href="/favicon.ico" />
      </Helmet>,
    );
    await waitFor(() => {
      const link = document.querySelector('link[data-helmet][rel="icon"]');
      expect(link).toBeTruthy();
      expect(link?.getAttribute('href')).toBe('/favicon.ico');
    });
  });

  it('maps charSet prop to charset attribute', async () => {
    render(
      <Helmet>
        <meta charSet="utf-8" />
      </Helmet>,
    );
    await waitFor(() => {
      expect(document.querySelector('meta[charset="utf-8"]')).toBeTruthy();
    });
  });

  it('maps httpEquiv prop to http-equiv attribute', async () => {
    render(
      <Helmet>
        <meta httpEquiv="X-UA-Compatible" content="IE=edge" />
      </Helmet>,
    );
    await waitFor(() => {
      expect(document.querySelector('meta[http-equiv="X-UA-Compatible"]')).toBeTruthy();
    });
  });

  it('handles boolean attributes (e.g. async on script)', async () => {
    render(
      <Helmet>
        <script src="/app.js" async />
      </Helmet>,
    );
    await waitFor(() => {
      const script = document.querySelector('script[src="/app.js"]');
      expect(script).toBeTruthy();
      expect(script?.hasAttribute('async')).toBe(true);
    });
  });

  it('sets textContent for <style> child', async () => {
    render(
      <Helmet>
        <style>{'body { margin: 0; }'}</style>
      </Helmet>,
    );
    await waitFor(() => {
      const style = document.querySelector('style[data-helmet]');
      expect(style?.textContent).toBe('body { margin: 0; }');
    });
  });

  it('removes managed nodes on unmount', async () => {
    const {unmount} = render(
      <Helmet>
        <meta name="test-unmount" content="bye" />
      </Helmet>,
    );
    await waitFor(() => {
      expect(document.querySelector('meta[name="test-unmount"]')).toBeTruthy();
    });
    unmount();
    expect(document.querySelector('meta[name="test-unmount"]')).toBeNull();
  });

  it('replaces nodes on re-render (no duplicate tags)', async () => {
    const {rerender} = render(
      <Helmet>
        <meta name="x-rerender" content="first" />
      </Helmet>,
    );
    await waitFor(() => {
      expect(document.querySelector('meta[name="x-rerender"]')?.getAttribute('content')).toBe('first');
    });

    rerender(
      <Helmet>
        <meta name="x-rerender" content="second" />
      </Helmet>,
    );
    await waitFor(() => {
      expect(document.querySelector('meta[name="x-rerender"]')?.getAttribute('content')).toBe('second');
      expect(document.querySelectorAll('meta[name="x-rerender"]').length).toBe(1);
    });
  });

  it('handles multiple children simultaneously', async () => {
    render(
      <Helmet>
        <title>Multi Test</title>
        <meta name="author" content="Team" />
        <link rel="canonical" href="https://example.com" />
      </Helmet>,
    );
    await waitFor(() => {
      expect(document.title).toBe('Multi Test');
      expect(document.querySelector('meta[name="author"]')).toBeTruthy();
      expect(document.querySelector('link[rel="canonical"]')).toBeTruthy();
    });
  });

  it('ignores non-element children', async () => {
    const headChildCountBefore = document.head.children.length;
    render(<Helmet>{'just a string'}</Helmet>);
    await waitFor(() => {
      expect(document.head.children.length).toBe(headChildCountBefore);
    });
  });
});
