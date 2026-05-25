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

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import NamespaceSelector from '@/components/edit-translation/NamespaceSelector';

const defaultProps = {
  namespaces: ['commonNamespace', 'loginFlow', 'userProfile'],
  value: 'commonNamespace',
  loading: false,
  onChange: vi.fn(),
};

describe('NamespaceSelector', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('renders the namespace label', () => {
      render(<NamespaceSelector {...defaultProps} />);

      expect(screen.getByText('Namespace')).toBeInTheDocument();
    });

    it('renders the helper text', () => {
      render(<NamespaceSelector {...defaultProps} />);

      expect(
        screen.getByText(
          'A namespace typically represents a page or a section within a page. It helps group and organize related translation keys for better structure and maintainability.',
        ),
      ).toBeInTheDocument();
    });

    it('renders with the current value displayed in the input', () => {
      render(<NamespaceSelector {...defaultProps} value="loginFlow" />);

      expect(screen.getByRole('combobox')).toHaveValue('Login Flow');
    });

    it('renders with empty string when value is null', () => {
      render(<NamespaceSelector {...defaultProps} value={null} />);

      expect(screen.getByRole('combobox')).toHaveValue('');
    });
  });

  describe('Option label formatting', () => {
    it('formats camelCase namespace keys into human-readable labels', async () => {
      const user = userEvent.setup();

      render(<NamespaceSelector {...defaultProps} />);

      await user.click(screen.getByRole('combobox'));

      expect(screen.getByText('Common Namespace')).toBeInTheDocument();
      expect(screen.getByText('Login Flow')).toBeInTheDocument();
      expect(screen.getByText('User Profile')).toBeInTheDocument();
    });
  });

  describe('Interaction', () => {
    it('calls onChange when a namespace option is selected', async () => {
      const onChange = vi.fn();
      const user = userEvent.setup();

      render(<NamespaceSelector {...defaultProps} onChange={onChange} />);

      await user.click(screen.getByRole('combobox'));
      await user.click(screen.getByText('Login Flow'));

      expect(onChange).toHaveBeenCalledWith('loginFlow');
    });
  });

  describe('Loading state', () => {
    it('shows no namespace options while loading', async () => {
      const user = userEvent.setup();

      render(<NamespaceSelector {...defaultProps} loading namespaces={[]} value={null} />);

      // MUI Autocomplete's loading indicator lives inside the Popper listbox,
      // which requires a real layout engine to position in jsdom. Assert the
      // observable behaviour instead: with no loaded namespaces there are no
      // selectable option elements.
      await user.click(screen.getByRole('combobox'));

      expect(screen.queryAllByRole('option')).toHaveLength(0);
    });

    it('does not show loading indicator when loading is false', () => {
      render(<NamespaceSelector {...defaultProps} loading={false} />);

      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    });
  });
});
