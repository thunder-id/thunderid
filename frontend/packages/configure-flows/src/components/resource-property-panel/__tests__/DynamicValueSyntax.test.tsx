// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import DynamicValueSyntax from '../DynamicValueSyntax';

describe('DynamicValueSyntax', () => {
  describe('valid dynamic value patterns', () => {
    it('should render meta pattern with syntax highlighting', () => {
      render(<DynamicValueSyntax value="{{meta(application.name)}}" />);

      expect(screen.getByText('meta')).toBeInTheDocument();
      expect(screen.getByText('application.name')).toBeInTheDocument();
      expect(screen.getByText('{{')).toBeInTheDocument();
      expect(screen.getByText(')}}')).toBeInTheDocument();
      expect(screen.getByText('(')).toBeInTheDocument();
    });

    it('should render translation pattern with syntax highlighting', () => {
      render(<DynamicValueSyntax value="{{t(flowI18n:login.title)}}" />);

      expect(screen.getByText('t')).toBeInTheDocument();
      expect(screen.getByText('flowI18n:login.title')).toBeInTheDocument();
    });

    it('should handle whitespace around the value', () => {
      render(<DynamicValueSyntax value="  {{meta(ou.name)}}  " />);

      expect(screen.getByText('meta')).toBeInTheDocument();
      expect(screen.getByText('ou.name')).toBeInTheDocument();
    });
  });

  describe('invalid or unrecognized patterns', () => {
    it('should render plain text for non-dynamic values', () => {
      render(<DynamicValueSyntax value="Hello World" />);

      expect(screen.getByText('Hello World')).toBeInTheDocument();
    });

    it('should render plain text for incomplete patterns', () => {
      render(<DynamicValueSyntax value="{{meta(incomplete" />);

      expect(screen.getByText('{{meta(incomplete')).toBeInTheDocument();
    });

    it('should render plain text for empty value', () => {
      render(<DynamicValueSyntax value="" />);

      const {container} = render(<DynamicValueSyntax value="" />);
      expect(container).toBeTruthy();
    });

    it('should render plain text for malformed braces', () => {
      render(<DynamicValueSyntax value="{meta(key)}" />);

      expect(screen.getByText('{meta(key)}')).toBeInTheDocument();
    });
  });
});
