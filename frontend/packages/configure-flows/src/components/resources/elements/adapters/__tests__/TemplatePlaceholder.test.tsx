// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import TemplatePlaceholder, {containsTemplateLiteral} from '../TemplatePlaceholder';

describe('TemplatePlaceholder', () => {
  describe('containsTemplateLiteral', () => {
    it('should return true for strings containing a template literal', () => {
      expect(containsTemplateLiteral('{{meta(name)}}')).toBe(true);
    });

    it('should return true for strings with template surrounded by text', () => {
      expect(containsTemplateLiteral('Hello {{meta(name)}} world')).toBe(true);
    });

    it('should return false for plain strings', () => {
      expect(containsTemplateLiteral('Hello world')).toBe(false);
    });

    it('should return false for undefined', () => {
      expect(containsTemplateLiteral(undefined)).toBe(false);
    });

    it('should return false for empty string', () => {
      expect(containsTemplateLiteral('')).toBe(false);
    });
  });

  describe('rendering', () => {
    it('should return value directly when there are no segments', () => {
      const {container} = render(<TemplatePlaceholder value="" />);
      expect(container).toBeEmptyDOMElement();
    });

    it('should render plain text without badges', () => {
      render(<TemplatePlaceholder value="Hello world" />);
      expect(screen.getByText('Hello world')).toBeInTheDocument();
    });

    it('should render a template segment as a styled badge', () => {
      render(<TemplatePlaceholder value="{{meta(name)}}" />);
      expect(screen.getByText('meta')).toBeInTheDocument();
      expect(screen.getByText('name')).toBeInTheDocument();
    });

    it('should render text before a template match', () => {
      render(<TemplatePlaceholder value="Hello {{meta(name)}}" />);
      expect(screen.getByText('Hello')).toBeInTheDocument();
      expect(screen.getByText('meta')).toBeInTheDocument();
      expect(screen.getByText('name')).toBeInTheDocument();
    });

    it('should render trailing text after a template match', () => {
      render(<TemplatePlaceholder value="{{meta(name)}} world" />);
      expect(screen.getByText('meta')).toBeInTheDocument();
      expect(screen.getByText('world')).toBeInTheDocument();
    });

    it('should render mixed content with text and multiple templates', () => {
      const {container} = render(<TemplatePlaceholder value="Hello {{meta(name)}} and {{meta(age)}} end" />);
      expect(container.textContent).toContain('Hello');
      expect(container.textContent).toContain('and');
      expect(container.textContent).toContain('end');
      expect(screen.getByText('name')).toBeInTheDocument();
      expect(screen.getByText('age')).toBeInTheDocument();
    });

    it('should resolve t() segments using the provided t prop', () => {
      const t = (key: string): string => (key === 'greeting' ? 'Hola' : key);
      render(<TemplatePlaceholder value="{{t(greeting)}}" t={t} />);
      expect(screen.getByText('Hola')).toBeInTheDocument();
    });

    it('should use key as fallback when t prop is not provided for t() segments', () => {
      render(<TemplatePlaceholder value="{{t(greeting)}}" />);
      expect(screen.getByText('greeting')).toBeInTheDocument();
    });

    it('should handle t() segments mixed with other templates and text', () => {
      const t = (key: string): string => (key === 'label' ? 'Name' : key);
      const {container} = render(<TemplatePlaceholder value="{{t(label)}}: {{meta(user)}}" t={t} />);
      expect(container.textContent).toContain('Name');
      expect(container.textContent).toContain(':');
      expect(screen.getByText('user')).toBeInTheDocument();
    });
  });
});
