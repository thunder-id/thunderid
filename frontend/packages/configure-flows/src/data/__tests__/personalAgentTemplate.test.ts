// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import templates from '../templates.json';

interface Component {
  id?: string;
  ref?: string;
  type?: string;
  source?: string;
  components?: Component[];
}

interface Node {
  id: string;
  type: string;
  next?: string;
  onSuccess?: string;
  onFailure?: string;
  onIncomplete?: string;
  flow?: {ref?: string};
  executor?: {name?: string; inputs?: {identifier: string; type?: string; defaultValue?: string; constant?: boolean}[]};
  properties?: Record<string, unknown>;
  meta?: {components?: Component[]};
  prompts?: {inputs?: {identifier: string; required?: boolean}[]; action?: {ref?: string; nextNode?: string}}[];
}

interface Template {
  type?: string;
  flowType?: string;
  config?: {handle?: string; nodes?: Node[]};
}

const template = (templates as Template[]).find(
  (candidate) => candidate.type === 'PERSONAL_AGENT' && candidate.flowType === 'REGISTRATION',
);
const nodes = new Map((template?.config?.nodes ?? []).map((node) => [node.id, node]));

const flatten = (components: Component[] = []): Component[] =>
  components.flatMap((component) => [component, ...flatten(component.components)]);

describe('personal assistant registration template', () => {
  it('should be offered for registration flows', () => {
    expect(template).toBeDefined();
    expect(template?.config?.handle).toBe('personal-assistant-registration-flow');
  });

  it('should start with the choice between signing up and registering a personal assistant', () => {
    const choice = nodes.get(nodes.get('start')?.onSuccess ?? '');

    expect(choice?.prompts?.map((prompt) => prompt.action?.nextNode)).toEqual([
      'user_type_resolver',
      'call_owner_login',
    ]);
  });

  it('should sign a user up with a username and password on the other choice', () => {
    const resolver = nodes.get('user_type_resolver');
    const credentials = nodes.get(resolver?.onSuccess ?? '');
    const provisioning = nodes.get(credentials?.prompts?.[0]?.action?.nextNode ?? '');

    expect(resolver?.executor?.name).toBe('UserTypeResolver');
    expect(credentials?.prompts?.[0]?.inputs).toEqual([
      expect.objectContaining({identifier: 'username', required: true}),
      expect.objectContaining({identifier: 'password', required: true}),
    ]);
    expect(provisioning?.executor?.name).toBe('ProvisioningExecutor');
    expect(provisioning?.executor?.inputs).toBeUndefined();
    expect(provisioning?.onSuccess).toBe('end');
  });

  it('should sign the owner in through the default personal agent authentication flow', () => {
    const call = nodes.get('call_owner_login');

    expect(call?.type).toBe('CALL');
    expect(call?.flow?.ref).toBe('01900000-0000-7000-8000-000000000092');
    expect(call?.onSuccess).toBe('agent_type_resolution');
  });

  it('should provision a personal agent, with its kind, login flow and consent fixed by the flow', () => {
    const provisioning = nodes.get('agent_provisioning');

    expect(provisioning?.executor?.name).toBe('ProvisioningExecutor');
    expect(provisioning?.properties).toEqual({includeOptional: true});
    expect(provisioning?.executor?.inputs).toEqual([
      expect.objectContaining({identifier: 'kind', defaultValue: 'personal-assistant', constant: true}),
      expect.objectContaining({
        identifier: 'authFlowId',
        defaultValue: '01900000-0000-7000-8000-000000000092',
        constant: true,
      }),
      expect.objectContaining({
        identifier: 'alwaysPromptConsent',
        type: 'BOOLEAN_INPUT',
        defaultValue: 'true',
        constant: true,
      }),
    ]);
  });

  it('should collect the agent details, including the schema attributes, when provisioning needs them', () => {
    const details = nodes.get(nodes.get('agent_provisioning')?.onIncomplete ?? '');
    const types = flatten(details?.meta?.components).map((component) => component.type);
    const refs = flatten(details?.meta?.components).map((component) => component.ref);

    expect(types).toContain('DYNAMIC_INPUT_PLACEHOLDER');
    expect(refs).toEqual(expect.arrayContaining(['redirectUris', 'jwksUri']));
    expect(refs).not.toContain('publicKey');
    expect(details?.prompts?.[0]?.action?.nextNode).toBe('agent_provisioning');
  });

  it('should send a failed registration back to the details prompt', () => {
    const provisioning = nodes.get('agent_provisioning');

    expect(provisioning?.onFailure).toBe(provisioning?.onIncomplete);
    expect(nodes.get(provisioning?.onFailure ?? '')?.type).toBe('PROMPT');
  });

  it('should require a JWKS URI, so the agent never gets a client secret', () => {
    const details = nodes.get(nodes.get('agent_provisioning')?.onIncomplete ?? '');
    const jwksUri = details?.prompts?.[0]?.inputs?.find((input) => input.identifier === 'jwksUri');

    expect(jwksUri?.required).toBe(true);
  });

  it('should show only the client ID once the agent is registered', () => {
    const credentials = nodes.get(nodes.get('agent_provisioning')?.onSuccess ?? '');
    const sources = flatten(credentials?.meta?.components)
      .map((component) => component.source)
      .filter(Boolean);

    expect(sources).toEqual(['clientId']);
    expect(credentials?.next).toBe('end');
  });

  it('should point every connection at a node that exists', () => {
    nodes.forEach((node) => {
      [
        node.next,
        node.onSuccess,
        node.onFailure,
        node.onIncomplete,
        ...(node.prompts ?? []).map((p) => p.action?.nextNode),
      ]
        .filter((target): target is string => Boolean(target))
        .forEach((target) => expect(nodes.has(target), `${node.id} -> ${target}`).toBe(true));
    });
  });
});
