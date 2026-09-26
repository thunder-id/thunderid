// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {
  ApplicationCreateFlowStep,
  ApplicationCreateFlowSignInApproach,
  ApplicationCreateFlowConfiguration,
} from '../application-create-flow';

describe('application-create-flow models', () => {
  describe('ApplicationCreateFlowStep', () => {
    it('should have DETAILS step', () => {
      expect(ApplicationCreateFlowStep.DETAILS).toBe('DETAILS');
    });

    it('should have SECURITY step', () => {
      expect(ApplicationCreateFlowStep.SECURITY).toBe('SECURITY');
    });

    it('should have DESIGN step', () => {
      expect(ApplicationCreateFlowStep.DESIGN).toBe('DESIGN');
    });

    it('should have ORGANIZATION_UNIT step', () => {
      expect(ApplicationCreateFlowStep.ORGANIZATION_UNIT).toBe('ORGANIZATION_UNIT');
    });

    it('should have CONFIGURE step', () => {
      expect(ApplicationCreateFlowStep.CONFIGURE).toBe('CONFIGURE');
    });

    it('should have COMPLETE step', () => {
      expect(ApplicationCreateFlowStep.COMPLETE).toBe('COMPLETE');
    });

    it('should have CLIENT_TYPE step', () => {
      expect(ApplicationCreateFlowStep.CLIENT_TYPE).toBe('CLIENT_TYPE');
    });

    it('should have exactly 7 steps', () => {
      expect(Object.keys(ApplicationCreateFlowStep)).toHaveLength(7);
    });
  });

  describe('ApplicationCreateFlowSignInApproach', () => {
    it('should have REDIRECT_BASED approach', () => {
      expect(ApplicationCreateFlowSignInApproach.REDIRECT_BASED).toBe('REDIRECT_BASED');
    });

    it('should have EMBEDDED approach', () => {
      expect(ApplicationCreateFlowSignInApproach.EMBEDDED).toBe('EMBEDDED');
    });

    it('should have exactly 2 approaches', () => {
      expect(Object.keys(ApplicationCreateFlowSignInApproach)).toHaveLength(2);
    });
  });

  describe('ApplicationCreateFlowConfiguration', () => {
    it('should have URL configuration', () => {
      expect(ApplicationCreateFlowConfiguration.URL).toBe('URL');
    });

    it('should have DEEPLINK configuration', () => {
      expect(ApplicationCreateFlowConfiguration.DEEPLINK).toBe('DEEPLINK');
    });

    it('should have NONE configuration', () => {
      expect(ApplicationCreateFlowConfiguration.NONE).toBe('NONE');
    });

    it('should have exactly 3 configurations', () => {
      expect(Object.keys(ApplicationCreateFlowConfiguration)).toHaveLength(3);
    });
  });
});
