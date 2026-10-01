// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import {ElementTypes} from '../../../models/elements';
import type {Resource} from '../../../models/resources';
import {WidgetTypes} from '../../../models/widget';
import CommonWidgetPropertyFactory from '../CommonWidgetPropertyFactory';

describe('CommonWidgetPropertyFactory', () => {
  describe('Default Behavior', () => {
    it('should return null for IdentifierPassword widget', () => {
      const resource: Resource = {
        id: 'widget-1',
        type: WidgetTypes.IdentifierPassword,
        config: {},
      } as Resource;

      const {container} = render(<CommonWidgetPropertyFactory resource={resource} />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null for GoogleFederation widget', () => {
      const resource: Resource = {
        id: 'widget-3',
        type: WidgetTypes.GoogleFederation,
        config: {},
      } as Resource;

      const {container} = render(<CommonWidgetPropertyFactory resource={resource} />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null for SMSOTP widget', () => {
      const resource: Resource = {
        id: 'widget-4',
        type: WidgetTypes.SMSOTP,
        config: {},
      } as Resource;

      const {container} = render(<CommonWidgetPropertyFactory resource={resource} />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null for GithubFederation widget', () => {
      const resource: Resource = {
        id: 'widget-8',
        type: WidgetTypes.GithubFederation,
        config: {},
      } as Resource;

      const {container} = render(<CommonWidgetPropertyFactory resource={resource} />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null for PasskeyAuthentication widget', () => {
      const resource: Resource = {
        id: 'widget-9',
        type: WidgetTypes.PasskeyAuthentication,
        config: {},
      } as Resource;

      const {container} = render(<CommonWidgetPropertyFactory resource={resource} />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null for Provisioning widget', () => {
      const resource: Resource = {
        id: 'widget-10',
        type: WidgetTypes.Provisioning,
        config: {},
      } as Resource;

      const {container} = render(<CommonWidgetPropertyFactory resource={resource} />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null for unknown widget type', () => {
      const resource: Resource = {
        id: 'widget-unknown',
        type: 'UNKNOWN_WIDGET',
        config: {},
      } as Resource;

      const {container} = render(<CommonWidgetPropertyFactory resource={resource} />);

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Props Handling', () => {
    it('should accept additional props without errors', () => {
      const resource: Resource = {
        id: 'widget-1',
        type: WidgetTypes.IdentifierPassword,
        config: {},
      } as Resource;

      const {container} = render(
        <CommonWidgetPropertyFactory resource={resource} customProp="value" anotherProp={123} />,
      );

      expect(container.firstChild).toBeNull();
    });

    it('should handle resource with complex config', () => {
      const resource: Resource = {
        id: 'widget-1',
        resourceType: 'WIDGET',
        type: WidgetTypes.IdentifierPassword,
        category: 'WIDGET',
        version: '1.0.0',
        deprecated: false,
        deletable: true,
        display: {
          label: 'Test Widget',
          image: '',
          showOnResourcePanel: false,
        },
        config: {
          field: {name: '', type: ElementTypes},
          styles: {},
          nested: {
            deep: {
              value: 'test',
            },
          },
          array: [1, 2, 3],
        },
      } as unknown as Resource;

      const {container} = render(<CommonWidgetPropertyFactory resource={resource} />);

      expect(container.firstChild).toBeNull();
    });
  });
});
