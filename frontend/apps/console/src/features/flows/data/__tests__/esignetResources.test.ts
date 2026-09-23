// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import VisualFlowConstants from '../../constants/VisualFlowConstants';
import {ExecutionTypes} from '../../models/steps';
import {WidgetTypes} from '../../models/widget';
import executors from '../executors.json';
import widgets from '../widgets.json';

/**
 * The flow builder's resource panel is driven by these static catalogues rather than by a backend
 * endpoint, so a typo in an executor name or widget type would silently produce a tile that
 * generates an invalid flow. These assertions pin the eSignet entries against the enums the rest
 * of the builder keys off.
 */
describe('eSignet flow builder resources', () => {
  const esignetExecutor = executors.find(
    (resource) => resource.data?.action?.executor?.name === ExecutionTypes.ESignetFederation,
  );

  it('offers an eSignet executor tile on the resource panel', () => {
    expect(esignetExecutor).toBeDefined();
    expect(esignetExecutor?.display?.label).toBe('eSignet Login');
    expect(esignetExecutor?.display?.showOnResourcePanel).toBe(true);
    expect(esignetExecutor?.type).toBe('TASK_EXECUTION');
  });

  it('seeds the executor tile with the connection-backed property shape', () => {
    // The backend validator rejects any property the executor does not declare, so these three
    // are exactly what ESignetOIDCExecutor supports.
    expect(esignetExecutor?.data?.properties).toEqual({
      idpId: '{{IDP_ID}}',
      allowAuthenticationWithoutLocalUser: false,
      allowRegistrationWithExistingUser: false,
    });
  });

  const esignetWidget = widgets.find((resource) => resource.type === WidgetTypes.ESignetFederation);

  it('offers a "Continue with eSignet" widget', () => {
    expect(esignetWidget).toBeDefined();
    expect(esignetWidget?.display?.label).toBe('Continue with eSignet');
    expect(esignetWidget?.display?.showOnResourcePanel).toBe(true);
  });

  it('wires the widget to the eSignet executor with its own step id replacer', () => {
    const serialized = JSON.stringify(esignetWidget);

    expect(serialized).toContain(ExecutionTypes.ESignetFederation);
    expect(serialized).toContain('ESIGNET_EXECUTION_STEP_ID');
    // A leftover Google replacer key would collide with a Google widget dropped in the same flow.
    expect(serialized).not.toContain('GOOGLE_EXECUTION_STEP_ID');
    expect(serialized).not.toContain('GoogleOIDCAuthExecutor');
  });

  it('allows the widget to be dropped on both the canvas and a view step', () => {
    expect(VisualFlowConstants.FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES).toContain(WidgetTypes.ESignetFederation);
    expect(VisualFlowConstants.FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES).toContain(WidgetTypes.ESignetFederation);
  });
});
