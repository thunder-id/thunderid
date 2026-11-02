// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createElement} from 'react';
import {describe, it, expect} from 'vitest';
import Notification, {NotificationType} from '../notification';
import type {Resource} from '../resources';

describe('Notification', () => {
  describe('NotificationType', () => {
    it('should have WARNING type', () => {
      expect(NotificationType.WARNING).toBe('warning');
    });

    it('should have ERROR type', () => {
      expect(NotificationType.ERROR).toBe('error');
    });

    it('should have INFO type', () => {
      expect(NotificationType.INFO).toBe('info');
    });
  });

  describe('Constructor', () => {
    it('should create notification with id, message, and type', () => {
      const notification = new Notification('notif-1', 'Test message', NotificationType.WARNING);

      expect(notification.getId()).toBe('notif-1');
      expect(notification.getMessage()).toBe('Test message');
      expect(notification.getType()).toBe('warning');
    });

    it('should create notification with ReactElement message', () => {
      const element = createElement('span', null, 'React message');
      const notification = new Notification('notif-2', element, NotificationType.ERROR);

      expect(notification.getId()).toBe('notif-2');
      expect(notification.getMessage()).toBe(element);
      expect(notification.getType()).toBe('error');
    });

    it('should initialize with empty resources', () => {
      const notification = new Notification('notif-3', 'Test', NotificationType.INFO);

      expect(notification.getResources()).toEqual([]);
      expect(notification.hasResources()).toBe(false);
    });
  });

  describe('getId', () => {
    it('should return the notification id', () => {
      const notification = new Notification('unique-id', 'Message', NotificationType.WARNING);

      expect(notification.getId()).toBe('unique-id');
    });
  });

  describe('getMessage', () => {
    it('should return string message', () => {
      const notification = new Notification('id', 'String message', NotificationType.WARNING);

      expect(notification.getMessage()).toBe('String message');
    });

    it('should return ReactElement message', () => {
      const element = createElement('div', {className: 'test'}, 'Content');
      const notification = new Notification('id', element, NotificationType.WARNING);

      expect(notification.getMessage()).toBe(element);
    });
  });

  describe('getType', () => {
    it('should return warning type', () => {
      const notification = new Notification('id', 'Message', NotificationType.WARNING);

      expect(notification.getType()).toBe('warning');
    });

    it('should return error type', () => {
      const notification = new Notification('id', 'Message', NotificationType.ERROR);

      expect(notification.getType()).toBe('error');
    });

    it('should return info type', () => {
      const notification = new Notification('id', 'Message', NotificationType.INFO);

      expect(notification.getType()).toBe('info');
    });
  });

  describe('Resource Management', () => {
    const mockResource: Resource = {
      id: 'resource-1',
      type: 'STEP',
      category: 'VIEW',
      display: {label: 'Test Resource', image: '', showOnResourcePanel: true},
    } as Resource;

    const mockResource2: Resource = {
      id: 'resource-2',
      type: 'STEP',
      category: 'EXECUTION',
      display: {label: 'Test Resource 2', image: '', showOnResourcePanel: true},
    } as Resource;

    describe('addResource', () => {
      it('should add a resource to the notification', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        notification.addResource(mockResource);

        expect(notification.hasResources()).toBe(true);
        expect(notification.getResources()).toHaveLength(1);
      });

      it('should add multiple resources', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        notification.addResource(mockResource);
        notification.addResource(mockResource2);

        expect(notification.getResources()).toHaveLength(2);
      });

      it('should overwrite resource with same id', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        const updatedResource = {...mockResource, category: 'UPDATED' as Resource['category']};

        notification.addResource(mockResource);
        notification.addResource(updatedResource);

        expect(notification.getResources()).toHaveLength(1);
        expect(notification.getResource('resource-1')?.category).toBe('UPDATED');
      });
    });

    describe('getResources', () => {
      it('should return empty array when no resources', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(notification.getResources()).toEqual([]);
      });

      it('should return array of resources', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        notification.addResource(mockResource);
        notification.addResource(mockResource2);

        const resources = notification.getResources();
        expect(resources).toHaveLength(2);
        expect(resources[0].id).toBe('resource-1');
        expect(resources[1].id).toBe('resource-2');
      });
    });

    describe('hasResources', () => {
      it('should return false when no resources', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(notification.hasResources()).toBe(false);
      });

      it('should return true when resources exist', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        notification.addResource(mockResource);

        expect(notification.hasResources()).toBe(true);
      });
    });

    describe('getResource', () => {
      it('should return undefined for non-existent resource', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(notification.getResource('non-existent')).toBeUndefined();
      });

      it('should return resource by id', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        notification.addResource(mockResource);

        const resource = notification.getResource('resource-1');
        expect(resource?.id).toBe('resource-1');
      });
    });

    describe('hasResource', () => {
      it('should return false for non-existent resource', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(notification.hasResource('non-existent')).toBe(false);
      });

      it('should return true for existing resource', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        notification.addResource(mockResource);

        expect(notification.hasResource('resource-1')).toBe(true);
      });
    });
  });

  describe('Panel Notification', () => {
    describe('setPanelNotification', () => {
      it('should set panel notification element', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        const element = createElement('div', null, 'Panel content');

        notification.setPanelNotification(element);

        expect(notification.hasPanelNotification()).toBe(true);
      });
    });

    describe('getPanelNotification', () => {
      it('should return undefined when not set', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(notification.getPanelNotification()).toBeUndefined();
      });

      it('should return panel notification element', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        const element = createElement('div', null, 'Panel content');

        notification.setPanelNotification(element);

        expect(notification.getPanelNotification()).toBe(element);
      });
    });

    describe('hasPanelNotification', () => {
      it('should return false when not set', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(notification.hasPanelNotification()).toBe(false);
      });

      it('should return true when set', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        notification.setPanelNotification(createElement('div'));

        expect(notification.hasPanelNotification()).toBe(true);
      });
    });
  });

  describe('Resource Field Notifications', () => {
    describe('addResourceFieldNotification', () => {
      it('should add field notification', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        notification.addResourceFieldNotification('email', 'Email is required');

        expect(notification.hasResourceFieldNotification('email')).toBe(true);
      });

      it('should overwrite existing field notification', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        notification.addResourceFieldNotification('email', 'Email is required');
        notification.addResourceFieldNotification('email', 'Invalid email format');

        expect(notification.getResourceFieldNotification('email')).toBe('Invalid email format');
      });
    });

    describe('getResourceFieldNotification', () => {
      it('should return empty string for non-existent field', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(notification.getResourceFieldNotification('non-existent')).toBe('');
      });

      it('should return field notification message', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        notification.addResourceFieldNotification('password', 'Password too short');

        expect(notification.getResourceFieldNotification('password')).toBe('Password too short');
      });
    });

    describe('hasResourceFieldNotification', () => {
      it('should return false for non-existent field', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(notification.hasResourceFieldNotification('non-existent')).toBe(false);
      });

      it('should return true for existing field', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        notification.addResourceFieldNotification('username', 'Required');

        expect(notification.hasResourceFieldNotification('username')).toBe(true);
      });
    });

    describe('removeResourceFieldNotification', () => {
      it('should remove field notification', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        notification.addResourceFieldNotification('field1', 'Error');

        notification.removeResourceFieldNotification('field1');

        expect(notification.hasResourceFieldNotification('field1')).toBe(false);
      });

      it('should not throw when removing non-existent field', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        expect(() => {
          notification.removeResourceFieldNotification('non-existent');
        }).not.toThrow();
      });
    });

    describe('getResourceFieldNotifications', () => {
      it('should return empty map when no field notifications', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);

        const fieldNotifications = notification.getResourceFieldNotifications();
        expect(fieldNotifications.size).toBe(0);
      });

      it('should return map of all field notifications', () => {
        const notification = new Notification('id', 'Message', NotificationType.WARNING);
        notification.addResourceFieldNotification('field1', 'Error 1');
        notification.addResourceFieldNotification('field2', 'Error 2');

        const fieldNotifications = notification.getResourceFieldNotifications();
        expect(fieldNotifications.size).toBe(2);
        expect(fieldNotifications.get('field1')).toBe('Error 1');
        expect(fieldNotifications.get('field2')).toBe('Error 2');
      });
    });
  });
});
