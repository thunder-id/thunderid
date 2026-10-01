// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {FlowTemplate} from '../../../models/templates';
import SelectFlowTemplate from '../SelectFlowTemplate';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
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

const githubTemplate: FlowTemplate = {
  resourceType: 'TEMPLATE',
  category: 'SOCIAL_LOGIN',
  type: 'GITHUB',
  flowType: 'AUTHENTICATION',
  display: {label: 'GitHub', description: 'Sign in with GitHub', image: 'github.svg', showOnResourcePanel: true},
  config: {name: '', handle: '', nodes: []},
};

const compositeTemplate: FlowTemplate = {
  resourceType: 'TEMPLATE',
  category: 'SOCIAL_LOGIN',
  type: 'GOOGLE_GITHUB',
  flowType: 'AUTHENTICATION',
  display: {
    label: 'Google + GitHub',
    description: 'Sign in with Google or GitHub',
    image: 'google-github.svg',
    showOnResourcePanel: true,
  },
  config: {name: '', handle: '', nodes: []},
};

const unknownIconTemplate: FlowTemplate = {
  resourceType: 'TEMPLATE',
  category: 'MFA',
  type: 'SOME_UNMAPPED_TYPE',
  flowType: 'AUTHENTICATION',
  display: {label: 'Custom flow', description: 'A flow with no mapped icon', image: '', showOnResourcePanel: true},
  config: {name: '', handle: '', nodes: []},
};

const mockTemplates: FlowTemplate[] = [
  blankTemplate,
  passwordTemplate,
  googleTemplate,
  githubTemplate,
  compositeTemplate,
  unknownIconTemplate,
];

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

    it('should render a GitHub brand icon for a GitHub template', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByText('GitHub')).toBeInTheDocument();
    });

    it('should render a composite icon row for a template with multiple icons', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByText('Google + GitHub')).toBeInTheDocument();
    });

    it('should fall back to a generic icon for a template with no mapped icon', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      expect(screen.getByText('Custom flow')).toBeInTheDocument();
      expect(screen.getAllByTestId('icon-lock').length).toBeGreaterThan(0);
    });
  });

  describe('Search', () => {
    it('should filter templates by label', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      fireEvent.change(screen.getByPlaceholderText('Search templates...'), {target: {value: 'google'}});

      expect(screen.getByText('Google')).toBeInTheDocument();
      expect(screen.queryByText('Username & Password')).not.toBeInTheDocument();
    });

    it('should filter templates by description', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      fireEvent.change(screen.getByPlaceholderText('Search templates...'), {target: {value: 'mapped icon'}});

      expect(screen.getByText('Custom flow')).toBeInTheDocument();
      expect(screen.queryByText('Username & Password')).not.toBeInTheDocument();
    });

    it('should show the no-results message when nothing matches the search', () => {
      render(<SelectFlowTemplate {...defaultProps} />);

      fireEvent.change(screen.getByPlaceholderText('Search templates...'), {target: {value: 'nonexistent-xyz'}});

      expect(screen.getByText('No templates match your search.')).toBeInTheDocument();
    });
  });

  describe('Flow type changes', () => {
    it('should reset search and category filters when the flow type changes', () => {
      const {rerender} = render(<SelectFlowTemplate {...defaultProps} />);

      fireEvent.click(screen.getByText('Password'));
      fireEvent.change(screen.getByPlaceholderText('Search templates...'), {target: {value: 'google'}});

      rerender(<SelectFlowTemplate {...defaultProps} flowType="REGISTRATION" />);

      expect(screen.getByPlaceholderText('Search templates...')).toHaveValue('');
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

    it('should call onTemplateChange when Enter is pressed on a template card', () => {
      render(<SelectFlowTemplate {...defaultProps} selectedTemplate={blankTemplate} />);

      fireEvent.keyDown(screen.getByText('Username & Password').closest('[role="button"]')!, {key: 'Enter'});

      expect(mockOnTemplateChange).toHaveBeenCalledWith(passwordTemplate);
    });

    it('should call onTemplateChange when Space is pressed on a template card', () => {
      render(<SelectFlowTemplate {...defaultProps} selectedTemplate={blankTemplate} />);

      fireEvent.keyDown(screen.getByText('Username & Password').closest('[role="button"]')!, {key: ' '});

      expect(mockOnTemplateChange).toHaveBeenCalledWith(passwordTemplate);
    });

    it('should ignore unrelated key presses on a template card', () => {
      render(<SelectFlowTemplate {...defaultProps} selectedTemplate={blankTemplate} />);

      fireEvent.keyDown(screen.getByText('Username & Password').closest('[role="button"]')!, {key: 'a'});

      expect(mockOnTemplateChange).not.toHaveBeenCalled();
    });
  });
});
