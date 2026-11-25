// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import templates from '../templates.json';

interface Prompt {
  action?: {ref?: string; nextNode?: string};
}

interface Component {
  id: string;
  type?: string;
  eventType?: string;
  components?: Component[];
}

interface Node {
  id: string;
  type: string;
  onSuccess?: string;
  onFailure?: string;
  next?: string;
  properties?: Record<string, unknown>;
  meta?: {components?: Component[]};
  executor?: {name: string; mode?: string; inputs?: {identifier: string}[]};
  flow?: {ref: string; filterFlowType?: string};
  prompts?: Prompt[];
}

interface Template {
  category?: string;
  type?: string;
  flowType?: string;
  configureBeforeSave?: boolean;
  config?: {nodes?: Node[]};
}

const cibaTemplates = (templates as Template[]).filter((template) => template.flowType === 'CIBA');

/** Every component of a step, however deeply it is nested in blocks. */
const flatten = (components: Component[] = []): Component[] =>
  components.flatMap((component) => [component, ...flatten(component.components)]);

describe('CIBA templates', () => {
  it('has an email and an SMS template, in the CIBA category', () => {
    expect(cibaTemplates.map((template) => template.type)).toEqual([
      'CIBA_EMAIL_NOTIFICATION',
      'CIBA_SMS_NOTIFICATION',
    ]);
    expect(cibaTemplates.every((template) => template.category === 'CIBA')).toBe(true);
  });

  it('lists no CIBA template under another flow type', () => {
    const elsewhere = (templates as Template[]).filter((template) => template.flowType !== 'CIBA');

    expect(elsewhere.some((template) => template.type?.startsWith('CIBA_'))).toBe(false);
  });

  it('lists no CIBA template under Passwordless', () => {
    const passwordless = (templates as Template[]).filter((template) => template.category === 'PASSWORDLESS');

    expect(passwordless.some((template) => template.type?.startsWith('CIBA_'))).toBe(false);
  });

  it('differs between the channels only in the notification step and its icon', () => {
    const [email, sms] = cibaTemplates.map((template) => template.config?.nodes ?? []);
    // Sets aside the notification step, and the icon of the screen that follows it, which names the channel.
    const withoutChannel = (nodes: Node[]): Node[] =>
      nodes
        .filter((node) => !node.id.startsWith('send_notification'))
        .map((node) => {
          if (node.id === 'generate_invite') return {...node, onSuccess: 'notify'};
          if (node.id === 'notification_sent_status') {
            return {...node, meta: {components: node.meta?.components?.filter((c) => c.id !== 'notification_icon')}};
          }
          return node;
        });

    expect(withoutChannel(sms)).toEqual(withoutChannel(email));
  });

  describe.each(cibaTemplates)('$type', (template) => {
    const nodes = template.config?.nodes ?? [];
    const byId = (id?: string): Node | undefined => nodes.find((node) => node.id === id);
    const call = nodes.find((node) => node.type === 'CALL');
    const approve = byId('approve_request');
    const nextOf = (node?: Node): string | undefined => node?.onSuccess ?? node?.next;

    it('is a CIBA flow the builder opens before it is saved', () => {
      expect(template.flowType).toBe('CIBA');
      expect(template.configureBeforeSave).toBe(true);
    });

    it('calls an authentication flow chosen in the builder, with none chosen yet', () => {
      expect(call?.flow).toEqual({ref: '', filterFlowType: 'AUTHENTICATION'});
    });

    it('leaves sign-in, authorization and the assertion to the called flow', () => {
      const executors = nodes.map((node) => node.executor?.name);

      expect(executors).not.toContain('CredentialsAuthExecutor');
      expect(executors).not.toContain('AuthorizationExecutor');
      expect(executors).not.toContain('AuthAssertExecutor');
    });

    it('asks the user to approve or deny once the invite is verified', () => {
      expect(byId('verify_invite')?.onSuccess).toBe(approve?.id);
      expect(approve?.type).toBe('PROMPT');
      expect(approve?.prompts).toHaveLength(2);
    });

    it('renders a submit button for each approve and deny action', () => {
      const components = flatten(approve?.meta?.components);

      approve?.prompts?.forEach((prompt) => {
        const button = components.find((component) => component.id === prompt.action?.ref);

        expect(button?.type, `${prompt.action?.ref} has a button`).toBe('ACTION');
        expect(button?.eventType).toBe('SUBMIT');
      });
    });

    it('calls the flow when the user approves', () => {
      const action = approve?.prompts?.find((prompt) => prompt.action?.ref === 'action_approve')?.action;

      expect(action?.nextNode).toBe(call?.id);
    });

    it('shows a message and ends when the user denies, without calling the flow', () => {
      const action = approve?.prompts?.find((prompt) => prompt.action?.ref === 'action_deny')?.action;
      const denied = byId(action?.nextNode);

      expect(denied?.type).toBe('PROMPT');
      expect(denied?.id).not.toBe(call?.id);
      expect(byId(nextOf(denied))?.type).toBe('END');
    });

    it('sends the approval link through its own channel and shows the notification screens', () => {
      const notify = nodes.find((node) => node.id.startsWith('send_notification'));
      const channel = template.type === 'CIBA_SMS_NOTIFICATION' ? 'SMSExecutor' : 'EmailExecutor';

      expect(notify?.executor?.name).toBe(channel);
      expect(notify?.executor?.mode).toBe('send');
      expect(byId('generate_invite')?.onSuccess).toBe(notify?.id);
      expect(byId(notify?.onSuccess)?.id).toBe('notification_sent_status');
      expect(byId(notify?.onFailure)?.id).toBe('notification_error_status');
      // The executor reads only its own channel's property, so the other one must not stand in for it.
      const [templateProperty, otherProperty] =
        template.type === 'CIBA_SMS_NOTIFICATION' ? ['smsTemplate', 'emailTemplate'] : ['emailTemplate', 'smsTemplate'];

      expect(notify?.properties?.[templateProperty]).toBe('ciba-notification');
      expect(notify?.properties?.[otherProperty]).toBeUndefined();
    });

    it('asks an SMS template for a sender and a mobile number, and an email template for an email address', () => {
      const notify = nodes.find((node) => node.id.startsWith('send_notification'));
      const inputs = notify?.executor?.inputs?.map((input) => input.identifier);

      if (template.type === 'CIBA_SMS_NOTIFICATION') {
        expect(notify?.properties?.['senderId']).toBe('{{SENDER_ID}}');
        expect(inputs).toEqual(['mobile_number']);
      } else {
        expect(notify?.properties?.['senderId']).toBeUndefined();
        expect(inputs).toEqual(['email']);
      }
    });

    it('shows the success screen and ends once the called flow returns', () => {
      const success = byId(call?.onSuccess);

      expect(success?.type).toBe('PROMPT');
      expect(byId(nextOf(success))?.type).toBe('END');
    });
  });
});
