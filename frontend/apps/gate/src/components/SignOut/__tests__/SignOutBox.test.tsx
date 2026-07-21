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

import {act} from '@testing-library/react';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import SignOutBox from '../SignOutBox';

const {mockLogger} = vi.hoisted(() => ({
  mockLogger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

vi.mock('@thunderid/logger/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/logger/react')>();
  return {
    ...actual,
    useLogger: () => mockLogger,
  };
});

// Mock useDesign + layout/renderer so the box renders without the real design context.
const mockUseDesign = vi.fn();
let capturedOnSubmit: ((action: {id?: string}, inputs: Record<string, string>) => void) | undefined;
vi.mock('@thunderid/design', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/design')>();
  return {
    ...actual,
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    useDesign: () => mockUseDesign(),
    AuthCardLayout: ({children}: {children: React.ReactNode}) => <div data-testid="auth-card-layout">{children}</div>,
    FlowComponentRenderer: ({
      component,
      onSubmit,
    }: {
      component: {id?: string; type: string};
      onSubmit: (action: {id?: string}, inputs: Record<string, string>) => void;
    }) => {
      capturedOnSubmit = onSubmit;
      return (
        <div data-testid={`flow-component-${component.id ?? component.type}`}>{component.id ?? component.type}</div>
      );
    },
  };
});

// Mock useConfig
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.example.com');
vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getServerUrl: mockGetServerUrl,
  }),
}));

// Mock react-router hooks
let mockSearchParams = new URLSearchParams();
vi.mock('react-router', () => ({
  useSearchParams: () => [mockSearchParams],
}));

// Mock the SDK: useThunderID + a controllable normalizeFlowResponse.
const mockNormalize = vi.fn().mockReturnValue({components: [], additionalData: {}, executionId: ''});
vi.mock('@thunderid/react', async () => {
  const actual = await vi.importActual('@thunderid/react');
  return {
    ...actual,
    useThunderID: () => ({resolveFlowTemplateLiterals: (template: string) => template}),
    normalizeFlowResponse: (...args: unknown[]) => mockNormalize(...args) as {components: unknown[]},
  };
});

const assignSpy = vi.fn();

describe('SignOutBox', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseDesign.mockReturnValue({isDesignEnabled: false, isLoading: false});
    mockGetServerUrl.mockReturnValue('https://api.example.com');
    mockSearchParams = new URLSearchParams({executionId: 'exec-1', logoutId: 'logout-1'});
    mockNormalize.mockReturnValue({components: [], additionalData: {}, executionId: ''});
    capturedOnSubmit = undefined;
    // Default: an interactive step so the mount fetch neither redirects nor errors.
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ok: true, json: () => Promise.resolve({flowStatus: 'PROMPT', challengeToken: ''})}),
    );
    Object.defineProperty(window, 'location', {value: {href: ''}, writable: true, configurable: true});
    Object.defineProperty(window.location, 'href', {set: assignSpy, configurable: true});
  });

  it('renders the AuthCardLayout', async () => {
    render(<SignOutBox />);
    expect(await screen.findByTestId('auth-card-layout')).toBeInTheDocument();
  });

  it('resumes the execution against /flow/execute on mount', async () => {
    render(<SignOutBox />);
    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith(
        'https://api.example.com/flow/execute',
        expect.objectContaining({method: 'POST', credentials: 'include'}) as RequestInit,
      );
    });
    const body = JSON.parse((vi.mocked(fetch).mock.calls[0][1] as {body: string}).body) as {executionId?: string};
    expect(body.executionId).toBe('exec-1');
  });

  it('completes via the logout callback and redirects to the returned URI', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string) => {
        if (url.endsWith('/oauth2/logout/callback')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({redirect_uri: 'https://rp.example/after?state=xyz'}),
          });
        }
        return Promise.resolve({ok: true, json: () => Promise.resolve({flowStatus: 'COMPLETE'})});
      }),
    );
    render(<SignOutBox />);
    await waitFor(() => {
      expect(assignSpy).toHaveBeenCalledWith('https://rp.example/after?state=xyz');
    });
    // The callback was posted with the logout id from the URL.
    const callbackCall = vi.mocked(fetch).mock.calls.find((c) => (c[0] as string).endsWith('/oauth2/logout/callback'));
    expect(callbackCall).toBeDefined();
    const body = JSON.parse((callbackCall?.[1] as {body: string}).body) as {logoutId?: string};
    expect(body.logoutId).toBe('logout-1');
  });

  it('does not redirect when the logout callback returns no redirect_uri', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string) => {
        if (url.endsWith('/oauth2/logout/callback')) {
          return Promise.resolve({ok: true, json: () => Promise.resolve({})});
        }
        return Promise.resolve({ok: true, json: () => Promise.resolve({flowStatus: 'COMPLETE'})});
      }),
    );
    render(<SignOutBox />);
    await waitFor(() => {
      expect(vi.mocked(fetch).mock.calls.some((c) => (c[0] as string).endsWith('/oauth2/logout/callback'))).toBe(true);
    });
    expect(assignSpy).not.toHaveBeenCalled();
  });

  it('renders the confirmation step components for an interactive flow', async () => {
    mockNormalize.mockReturnValue({components: [{id: 'confirm', type: 'ACTION'}], additionalData: {}, executionId: ''});
    render(<SignOutBox />);
    expect(await screen.findByTestId('flow-component-confirm')).toBeInTheDocument();
  });

  it('echoes the challenge token from the prompt step on the next submit', async () => {
    const mockFetch = vi
      .fn()
      .mockResolvedValueOnce({ok: true, json: () => Promise.resolve({flowStatus: 'PROMPT', challengeToken: 'ct-1'})})
      .mockResolvedValueOnce({ok: true, json: () => Promise.resolve({flowStatus: 'COMPLETE'})})
      // The COMPLETE response triggers the logout completion callback.
      .mockResolvedValue({ok: true, json: () => Promise.resolve({})});
    vi.stubGlobal('fetch', mockFetch);
    mockNormalize.mockReturnValue({components: [{id: 'confirm', type: 'ACTION'}], additionalData: {}, executionId: ''});

    render(<SignOutBox />);
    await screen.findByTestId('flow-component-confirm');

    expect(capturedOnSubmit).toBeDefined();
    await act(async () => {
      capturedOnSubmit?.({id: 'action_confirm'}, {});
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(mockFetch.mock.calls.length).toBeGreaterThanOrEqual(2);
    });
    const submitBody = JSON.parse((mockFetch.mock.calls[1][1] as {body: string}).body) as {
      challengeToken?: string;
      action?: string;
    };
    expect(submitBody.challengeToken).toBe('ct-1');
    expect(submitBody.action).toBe('action_confirm');
  });

  it('shows an error alert and logs when the flow execute call fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ok: false, status: 500}));
    render(<SignOutBox />);
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(mockLogger.error).toHaveBeenCalled();
  });

  it('falls back to VITE_THUNDER_BASE_URL when getServerUrl returns null', async () => {
    mockGetServerUrl.mockReturnValue(null);
    render(<SignOutBox />);
    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith(
        `${import.meta.env.VITE_THUNDER_BASE_URL as string}/flow/execute`,
        expect.objectContaining({method: 'POST'}) as RequestInit,
      );
    });
  });
});
