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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {FlowTemplate} from '../../../models/templates';
import SelectFlowTemplate from '../SelectFlowTemplate';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue: string) => defaultValue,
  }),
}));

// Mock useColorScheme
vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual('@wso2/oxygen-ui');
  return {
    ...actual,
    useColorScheme: () => ({mode: 'light', systemMode: 'light'}),
  };
});

// Mock oxygen-ui-icons-react
vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Lock: ({size}: {size: number}) => <span data-testid="icon-lock">{size}</span>,
    Plus: ({size}: {size: number}) => <span data-testid="icon-plus">{size}</span>,
    Search: ({size}: {size: number}) => <span data-testid="icon-search">{size}</span>,
  };
});

// Mock resolveStaticResourcePath
vi.mock('../../../utils/resolveStaticResourcePath', () => ({
  default: (path: string) => `/static/${path}`,
}));

const blankTemplate: FlowTemplate = {
  resourceType: 'TEMPLATE',
  category: 'STARTER',
  type: 'BLANK',
  flowType: 'AUTHENTICATION',
  display: {label: 'Blank', description: 'Start from scratch', image: 'blank.svg', showOnResourcePanel: true},
  config: {name: '', handle: '', nodes: []},
};

const passwordTemplate: FlowTemplate = {
  resourceType: 'TEMPLATE',
  category: 'PASSWORD',
  type: 'CREDENTIALS_AUTH',
  flowType: 'AUTHENTICATION',
  display: {
    label: 'Username & Password',
    description: 'Basic authentication',
    image: 'basic.svg',
    showOnResourcePanel: true,
  },
  config: {name: '', handle: '', nodes: []},
};

const googleTemplate: FlowTemplate = {
  resourceType: 'TEMPLATE',
  category: 'SOCIAL_LOGIN',
  type: 'GOOGLE',
  flowType: 'AUTHENTICATION',
  display: {label: 'Google', description: 'Sign in with Google', image: 'google.svg', showOnResourcePanel: true},
  config: {name: '', handle: '', nodes: []},
};

const mockTemplates: FlowTemplate[] = [blankTemplate, passwordTemplate, googleTemplate];

vi.mock('../../../api/useGetFlowsMeta', () => ({
  default: () => ({
    data: {
      templates: mockTemplates,
      steps: [],
      actions: [],
      elements: [],
      widgets: [],
      executors: [],
    },
    error: null,
    isLoading: false,
  }),
}));

describe('SelectFlowTemplate', () => {
  const mockOnTemplateChange = vi.fn();

  const defaultProps = {
    flowType: 'AUTHENTICATION' as const,
    selectedTemplate: null,
    onTemplateChange: mockOnTemplateChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the component with data-testid', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByTestId('select-flow-template')).toBeInTheDocument();
    });

    it('should render the title', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByText('Choose a starting template')).toBeInTheDocument();
    });

    it('should render the blank template prominently as "Start from scratch"', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByText('Start from scratch')).toBeInTheDocument();
      expect(screen.getByText('Build your flow from the ground up with an empty canvas')).toBeInTheDocument();
    });

    it('should render non-blank templates in the grid', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByText('Username & Password')).toBeInTheDocument();
      expect(screen.getByText('Google')).toBeInTheDocument();
    });
  });

  describe('Category Filters', () => {
    it('should render the All chip', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByText('All')).toBeInTheDocument();
    });

    it('should render category chips for present categories', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByText('Password')).toBeInTheDocument();
      expect(screen.getByText('Social Login')).toBeInTheDocument();
    });

    it('should filter templates when a category chip is clicked', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      fireEvent.click(screen.getByText('Password'));

      expect(screen.getByText('Username & Password')).toBeInTheDocument();
      expect(screen.queryByText('Google')).not.toBeInTheDocument();
    });

    it('should show all non-blank templates when All chip is clicked after filtering', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      fireEvent.click(screen.getByText('Password'));
      fireEvent.click(screen.getByText('All'));

      expect(screen.getByText('Username & Password')).toBeInTheDocument();
      expect(screen.getByText('Google')).toBeInTheDocument();
    });
  });

  describe('Selection', () => {
    it('should auto-select the first template when no template is selected', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(mockOnTemplateChange).toHaveBeenCalledWith(blankTemplate);
    });

    it('should call onTemplateChange when a template card is clicked', () => {
      render(<SelectFlowTemplate {...defaultProps} selectedTemplate={blankTemplate} />);

      fireEvent.click(screen.getByText('Username & Password'));

      expect(mockOnTemplateChange).toHaveBeenCalledWith(passwordTemplate);
    });

    it('should call onTemplateChange when the blank template card is clicked', () => {
      render(<SelectFlowTemplate {...defaultProps} selectedTemplate={passwordTemplate} />);

      fireEvent.click(screen.getByText('Start from scratch'));

      expect(mockOnTemplateChange).toHaveBeenCalledWith(blankTemplate);
    });
  });
});
