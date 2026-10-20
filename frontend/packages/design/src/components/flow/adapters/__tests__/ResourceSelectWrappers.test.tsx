// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {cleanup, fireEvent, screen, waitFor} from '@testing-library/react';
import type {ComponentType} from 'react';
import {afterEach, describe, it, expect, vi, type Mock} from 'vitest';
import type {FlowComponent, FlowFieldProps} from '../../../../models/flow';
import type {FetchPage, PagedOption} from '../../../../test/pagedOptions';
import renderWithProviders from '../../../../test/renderWithProviders';
import {openList} from '../../../../test/selectInteractions';
import AuthFlowSelectAdapter from '../AuthFlowSelectAdapter';
import type {ResourceSelectAdapterProps} from '../ResourceSelectAdapter';
import UserSelectAdapter from '../UserSelectAdapter';
import UserTypeSelectAdapter from '../UserTypeSelectAdapter';

const loaders = vi.hoisted(() => ({
  authFlows: vi.fn<FetchPage>(),
  userTypes: vi.fn<FetchPage>(),
  users: vi.fn<FetchPage>(),
}));

vi.mock('@thunderid/react', async (importOriginal: () => Promise<Record<string, unknown>>) => ({
  ...(await importOriginal()),
  useFetchAuthFlows: () => loaders.authFlows,
  useFetchUserTypes: () => loaders.userTypes,
  useFetchUsers: () => loaders.users,
}));

afterEach(() => {
  cleanup();
  Object.values(loaders).forEach((loader) => loader.mockReset());
});

interface WrapperCase {
  name: string;
  Adapter: ComponentType<ResourceSelectAdapterProps>;
  component: FlowComponent;
  loader: Mock<FetchPage>;
  /** The input reference the picker reports changes under. */
  ref: string;
  /** What the picker submits for the first option, named for what it is. */
  submits: string;
  options: PagedOption[];
}

const cases: WrapperCase[] = [
  {
    name: 'UserSelectAdapter',
    Adapter: UserSelectAdapter,
    component: {id: 'owner_input', type: 'USER_SELECT', ref: 'owner', label: 'Owner', placeholder: 'Select an owner'},
    loader: loaders.users,
    ref: 'owner',
    options: [
      {label: 'Alice Admin', value: 'u-1'},
      {label: 'bob@example.com', value: 'u-2'},
    ],
    submits: 'id',
  },
  {
    name: 'AuthFlowSelectAdapter',
    Adapter: AuthFlowSelectAdapter,
    component: {
      id: 'flow_input',
      type: 'AUTH_FLOW_SELECT',
      ref: 'authFlowId',
      label: 'Authentication flow',
      placeholder: 'Select a flow',
    },
    loader: loaders.authFlows,
    ref: 'authFlowId',
    options: [
      {label: 'Default login', value: 'flow-1'},
      {label: 'mfa-login', value: 'flow-2'},
    ],
    submits: 'id',
  },
  {
    name: 'UserTypeSelectAdapter',
    Adapter: UserTypeSelectAdapter,
    component: {
      id: 'type_input',
      type: 'USER_TYPE_SELECT',
      ref: 'userType',
      label: 'User type',
      placeholder: 'Select a user type',
    },
    loader: loaders.userTypes,
    ref: 'userType',
    options: [
      {label: 'employee', value: 'employee'},
      {label: 'customer', value: 'customer'},
    ],
    submits: 'handle',
  },
];

const fieldProps = (component: FlowComponent, overrides: Partial<FlowFieldProps> = {}): FlowFieldProps => ({
  component,
  isLoading: false,
  onInputChange: vi.fn(),
  resolve: (s) => s,
  values: {},
  ...overrides,
});

describe.each(cases)('$name', ({Adapter, component, loader, options, ref, submits}) => {
  it('requests nothing while closed', () => {
    renderWithProviders(<Adapter {...fieldProps(component)} />);

    expect(loader).not.toHaveBeenCalled();
  });

  it('lists the options the SDK loader returns', async () => {
    loader.mockResolvedValue({nextOffset: null, options});
    renderWithProviders(<Adapter {...fieldProps(component)} />);

    openList();

    expect(await screen.findByRole('option', {name: options[0].label})).toBeTruthy();
    expect(screen.getByRole('option', {name: options[1].label})).toBeTruthy();
  });

  it(`submits the ${submits} of the picked option`, async () => {
    loader.mockResolvedValue({nextOffset: null, options: [options[0]]});
    const onInputChange = vi.fn();
    renderWithProviders(<Adapter {...fieldProps(component, {onInputChange})} />);

    openList();
    fireEvent.click(await screen.findByRole('option', {name: options[0].label}));

    expect(onInputChange).toHaveBeenCalledWith(ref, options[0].value);
  });

  it('requests pages of 30 from the first offset and passes the flow component', async () => {
    loader.mockResolvedValue({nextOffset: null, options: []});
    renderWithProviders(<Adapter {...fieldProps(component)} />);

    openList();

    await waitFor(() => expect(loader).toHaveBeenCalledTimes(1));
    expect(loader.mock.calls[0][0]).toMatchObject({component, limit: 30, offset: 0});
  });
});

describe('UserSelectAdapter filter', () => {
  it('sends the filter with the request', async () => {
    const {component} = cases[0];
    loaders.users.mockResolvedValue({nextOffset: null, options: []});
    renderWithProviders(<UserSelectAdapter {...fieldProps(component)} filter="department eq sales" />);

    openList();

    await waitFor(() => expect(loaders.users).toHaveBeenCalledTimes(1));
    expect(loaders.users.mock.calls[0][0]).toMatchObject({filter: 'department eq sales', limit: 30, offset: 0});
  });
});
