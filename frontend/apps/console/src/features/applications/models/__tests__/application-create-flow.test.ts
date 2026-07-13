/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {describe, it, expect} from 'vitest';
import {
  ApplicationCreateFlowStep,
  ApplicationCreateFlowSignInApproach,
  ApplicationCreateFlowConfiguration,
} from '../application-create-flow';

describe('application-create-flow models', () => {
  describe('ApplicationCreateFlowStep', () => {
    it('should have NAME step', () => {
      expect(ApplicationCreateFlowStep.NAME).toBe('NAME');
    });

    it('should have DESIGN step', () => {
      expect(ApplicationCreateFlowStep.DESIGN).toBe('DESIGN');
    });

    it('should have OPTIONS step', () => {
      expect(ApplicationCreateFlowStep.OPTIONS).toBe('OPTIONS');
    });

    it('should have EXPERIENCE step', () => {
      expect(ApplicationCreateFlowStep.EXPERIENCE).toBe('EXPERIENCE');
    });

    it('should have STACK step', () => {
      expect(ApplicationCreateFlowStep.STACK).toBe('STACK');
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

    it('should have exactly 9 steps', () => {
      expect(Object.keys(ApplicationCreateFlowStep)).toHaveLength(9);
    });
  });

  describe('ApplicationCreateFlowSignInApproach', () => {
    it('should have INBUILT approach', () => {
      expect(ApplicationCreateFlowSignInApproach.INBUILT).toBe('INBUILT');
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
