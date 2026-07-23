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

import {render, screen, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import HttpRequestProperties from '../HttpRequestProperties';
import type {Resource} from '@/features/flows/models/resources';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.httpRequest.description': 'Configure the HTTP Request step.',
        'flows:core.executions.httpRequest.url.label': 'URL',
        'flows:core.executions.httpRequest.url.placeholder': 'https://example.com',
        'flows:core.executions.httpRequest.method.label': 'Method',
        'flows:core.executions.httpRequest.headers.label': 'Headers',
        'flows:core.executions.httpRequest.headers.keyPlaceholder': 'Header name',
        'flows:core.executions.httpRequest.headers.valuePlaceholder': 'Header value',
        'flows:core.executions.httpRequest.body.label': 'Body',
        'flows:core.executions.httpRequest.body.placeholder': '{}',
        'flows:core.executions.httpRequest.timeout.label': 'Timeout',
        'flows:core.executions.httpRequest.timeout.placeholder': '10',
        'flows:core.executions.httpRequest.timeout.hint': 'Timeout in seconds (1-20)',
        'flows:core.executions.httpRequest.responseMapping.label': 'Response Mapping',
        'flows:core.executions.httpRequest.responseMapping.keyPlaceholder': 'Response key',
        'flows:core.executions.httpRequest.responseMapping.valuePlaceholder': 'Variable name',
        'flows:core.executions.httpRequest.errorHandling.label': 'Error Handling',
        'flows:core.executions.httpRequest.errorHandling.failOnError.label': 'Fail on error',
        'flows:core.executions.httpRequest.errorHandling.retryCount.label': 'Retry count',
        'flows:core.executions.httpRequest.errorHandling.retryCount.placeholder': '0',
        'flows:core.executions.httpRequest.errorHandling.retryCount.hint': 'Number of retries (0-5)',
        'flows:core.executions.httpRequest.errorHandling.retryDelay.label': 'Retry delay',
        'flows:core.executions.httpRequest.errorHandling.retryDelay.placeholder': '0',
        'flows:core.executions.httpRequest.errorHandling.retryDelay.hint': 'Delay between retries in ms (0-5000)',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('HttpRequestProperties', () => {
  const mockOnChange = vi.fn();

  const createResource = (properties: Record<string, unknown> = {}): Resource =>
    ({
      data: {
        properties,
      },
    }) as unknown as Resource;

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  describe('handleNumberPropertyChange with empty value', () => {
    it('should call onChange with min value when timeout is cleared', () => {
      const resource = createResource({timeout: 10});

      render(<HttpRequestProperties resource={resource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Timeout');
      fireEvent.change(timeoutInput, {target: {value: ''}});

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.timeout', 1, resource);
    });
  });

  describe('updateHeaderEntries', () => {
    it('should add a new empty header entry when add button is clicked', async () => {
      const user = userEvent.setup();
      const resource = createResource({headers: {}});

      render(<HttpRequestProperties resource={resource} onChange={mockOnChange} />);

      const addButtons = screen.getAllByLabelText('Add entry');
      // First "Add entry" button belongs to headers KeyValueEditor
      await user.click(addButtons[0]);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.headers', {'': ''}, resource);
    });

    it('should update a header value when the value field is edited', async () => {
      const user = userEvent.setup();
      const resource = createResource({headers: {'Content-Type': 'application/json', Accept: 'text/html'}});

      render(<HttpRequestProperties resource={resource} onChange={mockOnChange} />);

      const valueInputs = screen.getAllByPlaceholderText('Header value');
      // Edit the first header's value
      await user.clear(valueInputs[0]);
      await user.type(valueInputs[0], 'text/plain');
      await user.tab();

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.headers',
        {'Content-Type': 'text/plain', Accept: 'text/html'},
        resource,
      );
    });

    it('should remove a header entry when remove button is clicked', async () => {
      const user = userEvent.setup();
      const resource = createResource({headers: {'Content-Type': 'application/json', Accept: 'text/html'}});

      render(<HttpRequestProperties resource={resource} onChange={mockOnChange} />);

      const removeButtons = screen.getAllByLabelText('Remove entry');
      // Remove the first header entry
      await user.click(removeButtons[0]);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.headers', {Accept: 'text/html'}, resource);
    });
  });

  describe('updateResponseMappingEntries', () => {
    it('should add a new empty response mapping entry when add button is clicked', async () => {
      const user = userEvent.setup();
      const resource = createResource({responseMapping: {}});

      render(<HttpRequestProperties resource={resource} onChange={mockOnChange} />);

      const addButtons = screen.getAllByLabelText('Add entry');
      // Second "Add entry" button belongs to responseMapping KeyValueEditor
      await user.click(addButtons[1]);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.responseMapping', {'': ''}, resource);
    });

    it('should remove a response mapping entry when remove button is clicked', async () => {
      const user = userEvent.setup();
      const resource = createResource({responseMapping: {'data.id': 'userId', 'data.name': 'userName'}});

      render(<HttpRequestProperties resource={resource} onChange={mockOnChange} />);

      // There should be remove buttons for both headers (0 entries) and responseMapping (2 entries)
      const removeButtons = screen.getAllByLabelText('Remove entry');
      // Remove the first response mapping entry
      await user.click(removeButtons[0]);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.responseMapping', {'data.name': 'userName'}, resource);
    });
  });
});
