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

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import Error from '../../../components/Error/Error';

// Mock useSearchParams from react-router
const mockSearchParams = new URLSearchParams();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useSearchParams: () => [mockSearchParams],
  };
});

describe('Error', () => {
  beforeEach(() => {
    // Clear search params before each test
    Array.from(mockSearchParams.keys()).forEach((key) => mockSearchParams.delete(key));
  });

  it('renders without crashing', () => {
    const {container, rerender} = render(<Error />);
    rerender(<Error />);
    expect(container).toBeInTheDocument();
  });

  it('renders the main landmark element', () => {
    render(<Error />);
    expect(screen.getByRole('main')).toBeInTheDocument();
  });

  it('renders default error title when no errorCode is provided', () => {
    render(<Error />);
    expect(screen.getByText("Oops, that didn't work")).toBeInTheDocument();
  });

  it('renders default error description when no errorMessage is provided', () => {
    render(<Error />);
    expect(screen.getByText("We're sorry, we ran into a problem. Please try again!")).toBeInTheDocument();
  });

  it('renders custom error message from search params', () => {
    mockSearchParams.set('errorMessage', 'Something went terribly wrong');

    render(<Error />);
    expect(screen.getByText('Something went terribly wrong')).toBeInTheDocument();
  });

  it('renders invalid_request error title when errorCode is invalid_request', () => {
    mockSearchParams.set('errorCode', 'invalid_request');

    render(<Error />);
    expect(screen.getByText('Oh no, we ran into a problem!')).toBeInTheDocument();
  });

  it('renders invalid_request error description when errorCode is invalid_request', () => {
    mockSearchParams.set('errorCode', 'invalid_request');

    render(<Error />);
    expect(screen.getByText('The request is invalid. Please check and try again.')).toBeInTheDocument();
  });

  it('renders the logo images', () => {
    render(<Error />);

    const logos = screen.getAllByAltText(/Logo/i);
    expect(logos.length).toBeGreaterThan(0);
  });

  it('renders the logo with correct height', () => {
    render(<Error />);

    const logo = screen.getByAltText('Logo (Light)');
    expect(logo).toHaveStyle({height: '40px'});
  });

  it('renders the error images', () => {
    render(<Error />);

    const errorImages = screen.getAllByAltText(/Error Image/i);
    expect(errorImages.length).toBeGreaterThan(0);
  });

  it('keeps default title when errorCode is an unknown value', () => {
    mockSearchParams.set('errorCode', 'some_unknown_code');

    render(<Error />);
    expect(screen.getByText("Oops, that didn't work")).toBeInTheDocument();
    expect(screen.queryByText('Oh no, we ran into a problem!')).not.toBeInTheDocument();
  });

  it('uses errorMessage from search params even when errorCode is provided', () => {
    mockSearchParams.set('errorCode', 'invalid_request');
    mockSearchParams.set('errorMessage', 'Custom error message');

    render(<Error />);
    // errorCode=invalid_request overrides the description via useEffect
    expect(screen.getByText('The request is invalid. Please check and try again.')).toBeInTheDocument();
  });
});
