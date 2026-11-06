// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import type {TFunction} from 'i18next';
import {describe, it, expect, vi} from 'vitest';
import actions from '../../data/actions.json';
import elements from '../../data/elements.json';
import steps from '../../data/steps.json';
import rawTemplates from '../../data/templates.json';
import widgets from '../../data/widgets.json';
import type {FlowDefinitionResponse} from '../../models/responses';
import type {FlowTemplate} from '../../models/templates';
import {transformFlowToCanvas} from '../../utils/flowToCanvasTransformer';
import {transformReactFlow, validateFlowGraph} from '../../utils/reactFlowTransformer';
import {computeValidationNotifications} from '../../validation/computeValidationNotifications';
import {COMMON_GRAPH_VALIDATION_RULES, VALIDATION_RULES} from '../../validation/validation-rules';
import useGetFlowsMeta from '../useGetFlowsMeta';

const TEST_PRODUCT_NAME = 'TestProduct';

// Mock useConfig to avoid ConfigProvider requirement.
vi.mock('@thunderid/contexts', async (importOriginal) => {
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-type-assertion
  const actual = (await importOriginal()) as Record<string, unknown>;

  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {
          product_name: TEST_PRODUCT_NAME,
        },
      },
    }),
  };
});

describe('useGetFlowsMeta', () => {
  describe('Return Structure', () => {
    it('should return an object with data, error, and isLoading', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current).toHaveProperty('data');
      expect(result.current).toHaveProperty('error');
      expect(result.current).toHaveProperty('isLoading');
    });

    it('should return error as null', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.error).toBeNull();
    });

    it('should return isLoading as false', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.isLoading).toBe(false);
    });
  });

  describe('Data Content', () => {
    it('should return all templates when no flowType filter is provided', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.templates).toHaveLength((rawTemplates as FlowTemplate[]).length);
    });

    it('should substitute {{productName}} placeholders in templates with the configured product name', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      const serialised = JSON.stringify(result.current.data.templates);

      expect(serialised).not.toContain('{{productName}}');
      expect(serialised).toContain(TEST_PRODUCT_NAME);
    });

    it('should return actions from JSON file', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.actions).toEqual(actions);
    });

    it('should return elements from JSON file', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.elements).toEqual(elements);
    });

    it('should return steps from JSON file', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.steps).toEqual(steps);
    });

    it('should return widgets from JSON file', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.widgets).toEqual(widgets);
    });

    it('should include the required provisioning executor in registration blank template', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'REGISTRATION'}));

      const blankTemplate = result.current.data.templates.find((template) => template.type === 'BLANK');

      expect(blankTemplate?.config.nodes).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            id: 'start',
            onSuccess: 'user_type_resolver',
          }),
          expect.objectContaining({
            id: 'user_type_resolver',
            type: 'TASK_EXECUTION',
            executor: {name: 'UserTypeResolver'},
            onSuccess: 'view_prompt',
            onIncomplete: 'prompt_usertype',
          }),
          expect.objectContaining({
            id: 'provisioning',
            type: 'TASK_EXECUTION',
            executor: {name: 'ProvisioningExecutor'},
            onSuccess: 'END',
          }),
          expect.objectContaining({
            id: 'prompt_usertype',
            prompts: [
              expect.objectContaining({
                inputs: [
                  expect.objectContaining({
                    identifier: 'userType',
                    type: 'SELECT',
                    required: true,
                  }),
                ],
                action: {
                  ref: 'action_usertype',
                  nextNode: 'user_type_resolver',
                },
              }),
            ],
          }),
          expect.objectContaining({
            id: 'view_prompt',
            prompts: [
              expect.objectContaining({
                action: {
                  ref: 'action_continue',
                  nextNode: 'provisioning',
                },
              }),
            ],
          }),
        ]),
      );
    });

    it('should return an empty executors array', () => {
      const {result} = renderHook(() => useGetFlowsMeta());

      expect(result.current.data.executors).toEqual([]);
    });
  });

  describe('Filtering by flowType', () => {
    it('should filter templates by AUTHENTICATION flowType', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'AUTHENTICATION'}));

      const templates = result.current.data.templates;
      expect(templates.length).toBeGreaterThan(0);
      expect(templates.every((t: FlowTemplate) => t.flowType === 'AUTHENTICATION')).toBe(true);
    });

    it('should filter templates by REGISTRATION flowType', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'REGISTRATION'}));

      const templates = result.current.data.templates;
      expect(templates.length).toBeGreaterThan(0);
      expect(templates.every((t: FlowTemplate) => t.flowType === 'REGISTRATION')).toBe(true);
    });

    it('should provide the secure user deletion flow template', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'ADMINISTRATION'}));

      const template = result.current.data.templates.find((item) => item.type === 'USER_DELETION');
      expect(template?.display.label).toBe('User Deletion Flow');
      expect(template?.config.nodes.map((node) => node.executor?.name).filter(Boolean)).toEqual([
        'PermissionValidator',
        'PreDeleteExecutor',
        'CriteriaRevocationExecutor',
        'SessionRevocationExecutor',
        'UserDeleteExecutor',
      ]);
    });

    describe('Self templates', () => {
      type TemplateNode = FlowTemplate['config']['nodes'][number];

      const selfTemplate = (type: string): FlowTemplate => {
        const {result} = renderHook(() => useGetFlowsMeta({flowType: 'SELF'}));
        const template = result.current.data.templates.find((item) => item.type === type);
        expect(template).toBeDefined();
        return template!;
      };

      // Walks the graph from a node, taking a prompt's first action, and lists each step as
      // the executor and mode it runs or the prompt's id.
      const walk = (template: FlowTemplate, from: string): string[] => {
        const byId = new Map<string, TemplateNode>(template.config.nodes.map((node) => [node.id, node]));
        const steps: string[] = [];
        let current = byId.get(from);
        while (current && current.type !== 'END') {
          steps.push(current.executor ? `${current.executor.name} ${String(current.executor['mode'])}` : current.id);
          const next: string | undefined =
            current.onSuccess ?? current.next ?? current.prompts?.[0]?.action?.nextNode ?? undefined;
          current = next ? byId.get(next) : undefined;
        }
        return steps;
      };

      const node = (template: FlowTemplate, id: string): TemplateNode | undefined =>
        template.config.nodes.find((item) => item.id === id);

      it('should offer a blank and the self attribute verification templates for the Self type', () => {
        const {result} = renderHook(() => useGetFlowsMeta({flowType: 'SELF'}));

        expect(
          result.current.data.templates.map((template) => [
            template.type,
            template.display.label,
            template.config.name,
            template.config.handle,
          ]),
        ).toEqual([
          ['BLANK', 'Blank', 'New Self Flow', 'new-self-flow'],
          [
            'SELF_ATTRIBUTE_VERIFICATION',
            'Self Attribute Verification',
            'Self Attribute Verification Flow',
            'self-attribute-verification-flow',
          ],
          [
            'SELF_ATTRIBUTE_VERIFICATION_EMAIL_LINK',
            'Self Attribute Verification with Email Link',
            'Self Attribute Verification with Email Link Flow',
            'self-attribute-verification-email-link-flow',
          ],
        ]);
      });

      const EMAIL_FIRST_NODES = [
        ['SELF_ATTRIBUTE_VERIFICATION', 'generate_email_otp'],
        ['SELF_ATTRIBUTE_VERIFICATION_EMAIL_LINK', 'generate_link'],
      ];

      // The node the email branch starts with.
      const emailFirst = (template: FlowTemplate): string => node(template, 'prepare_verification')?.onSuccess ?? '';

      // The node the mobile branch starts with: where the email branch's condition skips to.
      const mobileFirst = (template: FlowTemplate): string =>
        node(template, emailFirst(template))?.condition?.onSkip ?? '';

      // Each node's outgoing links, for comparing a graph before and after the builder.
      const links = (nodes: TemplateNode[]): Record<string, unknown> =>
        Object.fromEntries(
          nodes.map((item) => [
            item.id,
            {
              onSuccess: item.onSuccess,
              onFailure: item.onFailure,
              onIncomplete: item.onIncomplete,
              next: item.next,
              actions: item.prompts?.map((entry) => entry.action?.nextNode),
            },
          ]),
        );

      it.each(EMAIL_FIRST_NODES)(
        'should prepare, then take the email branch only for an email address in %s',
        (type, first) => {
          const template = selfTemplate(type);

          expect(walk(template, 'start').slice(0, 2)).toEqual(['start', 'AttributeVerificationExecutor prepare']);
          expect(emailFirst(template)).toBe(first);
          // The pinned contact type picks the branch, so the client never names a channel.
          expect(node(template, first)?.condition).toEqual({
            key: '{{ctx(recipientType)}}',
            value: 'email',
            onSkip: 'generate_mobile_otp',
          });
          expect(template.config.nodes.filter((item) => item.condition).map((item) => item.id)).toEqual([first]);
          expect(template.config.nodes.filter((item) => item.type === 'PROMPT' && item.properties)).toEqual([]);
        },
      );

      // What follows the record in each template: the link template closes on a prompt, since Gate
      // shows only the views the flow returns.
      const AFTER_RECORD: [string, string[]][] = [
        ['SELF_ATTRIBUTE_VERIFICATION', []],
        ['SELF_ATTRIBUTE_VERIFICATION_EMAIL_LINK', ['verification_complete']],
      ];

      it.each(AFTER_RECORD)(
        'should verify a mobile number with an SMS code and record it in %s',
        (type, afterRecord) => {
          const template = selfTemplate(type);
          const mobile = mobileFirst(template);

          expect(walk(template, mobile)).toEqual([
            'OTPExecutor generate',
            'SMSExecutor send',
            'mobile_code',
            'OTPExecutor verify',
            'AttributeVerificationExecutor record',
            ...afterRecord,
          ]);
          // The builder replaces the placeholder when the flow loads, as for other SMS templates.
          expect(node(template, 'send_mobile_otp')?.properties).toEqual({
            senderId: '{{SENDER_ID}}',
            smsTemplate: 'otp',
          });
          // Resend pins again, so the new code goes to the current contact.
          expect(node(template, 'mobile_code')?.prompts?.map((entry) => entry.action?.nextNode)).toEqual([
            'verify_mobile_otp',
            'prepare_verification',
          ]);
        },
      );

      it('should verify an email address with an email code in the default template', () => {
        const template = selfTemplate('SELF_ATTRIBUTE_VERIFICATION');

        expect(walk(template, emailFirst(template))).toEqual([
          'OTPExecutor generate',
          'EmailExecutor send',
          'email_code',
          'OTPExecutor verify',
          'AttributeVerificationExecutor record',
        ]);
        expect(node(template, 'send_email_otp')?.properties).toEqual({emailTemplate: 'otp'});
        expect(node(template, 'email_code')?.prompts?.map((entry) => entry.action?.nextNode)).toEqual([
          'verify_email_otp',
          'prepare_verification',
        ]);
      });

      it('should verify an email address from a link confirmed before recording in the email link template', () => {
        const template = selfTemplate('SELF_ATTRIBUTE_VERIFICATION_EMAIL_LINK');

        expect(walk(template, emailFirst(template))).toEqual([
          'InviteExecutor generate',
          'EmailExecutor send',
          'link_sent',
          'InviteExecutor verify',
          'confirm',
          'AttributeVerificationExecutor record',
          'verification_complete',
        ]);
        expect(node(template, 'send_link')?.properties).toEqual({emailTemplate: 'self-email-verification'});
        // The app shows this while the user opens the email, so it has nothing to submit.
        const sent = node(template, 'link_sent');
        expect(sent?.prompts).toBeUndefined();
        expect((sent?.meta?.components as {label?: string}[]).map((component) => component.label)).toEqual([
          '{{ t(verification:self.email.link.sent.title) }}',
          '{{ t(verification:self.email.link.sent.message) }}',
        ]);
        // Gate draws only the views the flow returns, so the flow closes on one after recording.
        const complete = node(template, 'verification_complete');
        expect(complete?.prompts).toBeUndefined();
        expect(complete?.next).toBe('end');
        expect((complete?.meta?.components as {label?: string}[]).map((component) => component.label)).toEqual([
          '{{ t(verification:self.complete.title) }}',
          '{{ t(verification:self.complete.message) }}',
        ]);
      });

      it.each(EMAIL_FIRST_NODES)('should keep every link through a load and save of %s', (type) => {
        const template = selfTemplate(type);
        // The operator assigns the email and SMS providers before saving.
        const nodes = structuredClone(template.config.nodes).map((item) =>
          item.id.startsWith('send_') ? {...item, properties: {...item.properties, senderId: 'sender-1'}} : item,
        );

        const canvas = transformFlowToCanvas({nodes} as FlowDefinitionResponse);
        const saved = transformReactFlow(canvas);

        expect(links(saved.nodes as TemplateNode[])).toEqual(links(template.config.nodes));
        expect(validateFlowGraph(saved)).toEqual([]);
        expect([
          ...computeValidationNotifications(
            canvas.nodes,
            VALIDATION_RULES,
            ((key: string) => key) as TFunction,
            COMMON_GRAPH_VALIDATION_RULES,
            canvas.edges,
          ).keys(),
        ]).toEqual([]);
      });

      it.each(['SELF_ATTRIBUTE_VERIFICATION', 'SELF_ATTRIBUTE_VERIFICATION_EMAIL_LINK'])(
        'should ask for an SMS sender until one is assigned in %s',
        (type) => {
          const template = selfTemplate(type);
          const {nodes} = transformFlowToCanvas({nodes: template.config.nodes} as FlowDefinitionResponse);

          const notifications = computeValidationNotifications(
            nodes,
            VALIDATION_RULES,
            ((key: string) => key) as TFunction,
          );

          const notification = notifications.get('send_mobile_otp_REQUIRED_FIELD_ERROR');
          expect(notification?.hasResourceFieldNotification('send_mobile_otp_data.properties.senderId')).toBe(true);
        },
      );
    });

    describe('Administrator-initiated templates', () => {
      type TemplateNode = FlowTemplate['config']['nodes'][number];

      const adminTemplates = (): FlowTemplate[] => {
        const {result} = renderHook(() => useGetFlowsMeta({flowType: 'ADMIN_INITIATED'}));
        return result.current.data.templates;
      };

      it('should offer only the administrator attribute verification template', () => {
        expect(
          adminTemplates().map((template) => [
            template.type,
            template.display.label,
            template.config.name,
            template.config.handle,
          ]),
        ).toEqual([
          [
            'ADMIN_ATTRIBUTE_VERIFICATION_EMAIL_LINK',
            'Administrator Attribute Verification with Email Link',
            'Administrator Attribute Verification with Email Link Flow',
            'admin-attribute-verification-email-link-flow',
          ],
        ]);
      });

      it('should check the permission, prepare, send a link, confirm and record in order', () => {
        const [template] = adminTemplates();
        const byId = new Map<string, TemplateNode>(template.config.nodes.map((item) => [item.id, item]));
        const steps: string[] = [];
        let current = byId.get('start');
        while (current && current.type !== 'END') {
          steps.push(current.executor ? `${current.executor.name} ${String(current.executor['mode'])}` : current.id);
          const next: string | undefined =
            current.onSuccess ?? current.next ?? current.prompts?.[0]?.action?.nextNode ?? undefined;
          current = next ? byId.get(next) : undefined;
        }

        expect(steps).toEqual([
          'start',
          'PermissionValidator undefined',
          'AttributeVerificationExecutor prepare',
          'InviteExecutor generate',
          'EmailExecutor send',
          'link_sent',
          'InviteExecutor verify',
          'confirm',
          'AttributeVerificationExecutor record',
          'verification_complete',
        ]);
        expect(byId.get('permission_validator')?.properties).toEqual({requiredScopes: ['system:user']});
        expect(byId.get('send_link')?.properties).toEqual({emailTemplate: 'admin-email-verification'});
        // Email only: the administrator's request names an email address, so nothing branches.
        expect(template.config.nodes.filter((item) => item.condition)).toEqual([]);
      });

      it('should keep every link and need only an email sender through a load and save', () => {
        const [template] = adminTemplates();
        const nodes = structuredClone(template.config.nodes);

        const canvas = transformFlowToCanvas({nodes} as FlowDefinitionResponse);
        const saved = transformReactFlow(canvas);

        const links = (list: TemplateNode[]): unknown[] =>
          list.map((item) => [item.id, item.onSuccess, item.next, item.prompts?.map((e) => e.action?.nextNode)]);
        expect(links(saved.nodes as TemplateNode[])).toEqual(links(template.config.nodes));
        expect(validateFlowGraph(saved)).toEqual([]);
      });
    });

    it('should not affect non-template resources when filtering by flowType', () => {
      const {result} = renderHook(() => useGetFlowsMeta({flowType: 'AUTHENTICATION'}));

      expect(result.current.data.actions).toEqual(actions);
      expect(result.current.data.elements).toEqual(elements);
      expect(result.current.data.steps).toEqual(steps);
      expect(result.current.data.widgets).toEqual(widgets);
    });
  });

  describe('Memoization', () => {
    it('should return memoized data on subsequent renders', () => {
      const {result, rerender} = renderHook(() => useGetFlowsMeta());

      const initialData = result.current.data;
      rerender();

      expect(result.current.data).toBe(initialData);
    });

    it('should return new templates reference when flowType changes', () => {
      const {result, rerender} = renderHook<
        ReturnType<typeof useGetFlowsMeta>,
        {flowType: 'AUTHENTICATION' | 'REGISTRATION'}
      >(({flowType}) => useGetFlowsMeta({flowType}), {
        initialProps: {flowType: 'AUTHENTICATION'},
      });

      const firstTemplates = result.current.data.templates;
      rerender({flowType: 'REGISTRATION'});

      expect(result.current.data.templates).not.toBe(firstTemplates);
    });
  });
});
