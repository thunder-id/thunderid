// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {OrganizationUnitDefaultItem} from '../../../models/application-create-flow';
import type {OrganizationUnitDefaultsSelection} from '../../../models/application-create-flow';
import ConfigureApplicationDetails from '../ConfigureApplicationDetails';

interface MockOrganizationUnitQueryResult {
  data: OrganizationUnit | undefined;
  isLoading: boolean;
}

const mockUseGetOrganizationUnit = vi.fn<() => MockOrganizationUnitQueryResult>();

vi.mock('@thunderid/configure-organization-units', () => ({
  useGetOrganizationUnit: () => mockUseGetOrganizationUnit(),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string, fallback?: string) => fallback ?? key}),
}));

vi.mock('../ConfigureName', () => ({
  default: () => <div data-testid="configure-name-stub" />,
}));

const NO_DEFAULTS: OrganizationUnitDefaultsSelection = {
  [OrganizationUnitDefaultItem.SIGN_IN]: false,
  [OrganizationUnitDefaultItem.SIGN_UP]: false,
  [OrganizationUnitDefaultItem.RECOVERY]: false,
  [OrganizationUnitDefaultItem.SIGN_OUT]: false,
  [OrganizationUnitDefaultItem.THEME]: false,
  [OrganizationUnitDefaultItem.LAYOUT]: false,
};

const buildOu = (overrides: Partial<OrganizationUnit> = {}): OrganizationUnit => ({
  id: 'ou-1',
  handle: 'default',
  name: 'Default',
  authFlowId: 'flow-1',
  themeId: 'theme-1',
  ...overrides,
});

const baseProps = {
  hasMultipleOUs: false,
  selectedOuId: '',
  onChangeOu: vi.fn(),
  appName: 'My App',
  onAppNameChange: vi.fn(),
  appLogo: null,
  onLogoSelect: vi.fn(),
  hasSecurityStep: true,
  hasDesignStep: true,
  allowsUserLogins: true,
  userTypes: [],
  selectedUserTypes: [],
  onUserTypesChange: vi.fn(),
};

describe('ConfigureApplicationDetails', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('seeds ouDefaults with the available items once the OU resolves', () => {
    const onOuDefaultsChange = vi.fn();
    mockUseGetOrganizationUnit.mockReturnValue({data: buildOu(), isLoading: false});

    render(
      <ConfigureApplicationDetails
        {...baseProps}
        resolvedOuId="ou-1"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={onOuDefaultsChange}
      />,
    );

    expect(onOuDefaultsChange).toHaveBeenCalledTimes(1);
    expect(onOuDefaultsChange).toHaveBeenCalledWith({
      ...NO_DEFAULTS,
      [OrganizationUnitDefaultItem.SIGN_IN]: true,
      [OrganizationUnitDefaultItem.THEME]: true,
    });
  });

  it('does not seed the flows group when the template has no Security step', () => {
    const onOuDefaultsChange = vi.fn();
    mockUseGetOrganizationUnit.mockReturnValue({data: buildOu(), isLoading: false});

    render(
      <ConfigureApplicationDetails
        {...baseProps}
        hasSecurityStep={false}
        resolvedOuId="ou-1"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={onOuDefaultsChange}
      />,
    );

    expect(onOuDefaultsChange).toHaveBeenCalledWith({
      ...NO_DEFAULTS,
      [OrganizationUnitDefaultItem.THEME]: true,
    });
  });

  it('does not seed the design group when the template has no Design step', () => {
    const onOuDefaultsChange = vi.fn();
    mockUseGetOrganizationUnit.mockReturnValue({data: buildOu(), isLoading: false});

    render(
      <ConfigureApplicationDetails
        {...baseProps}
        hasDesignStep={false}
        resolvedOuId="ou-1"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={onOuDefaultsChange}
      />,
    );

    expect(onOuDefaultsChange).toHaveBeenCalledWith({
      ...NO_DEFAULTS,
      [OrganizationUnitDefaultItem.SIGN_IN]: true,
    });
  });

  it('does not render the ou defaults section when neither flows nor design are available for this template', () => {
    mockUseGetOrganizationUnit.mockReturnValue({data: buildOu(), isLoading: false});

    render(
      <ConfigureApplicationDetails
        {...baseProps}
        hasSecurityStep={false}
        hasDesignStep={false}
        resolvedOuId="ou-1"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={vi.fn()}
      />,
    );

    expect(screen.queryByTestId('application-configure-ou-defaults')).not.toBeInTheDocument();
  });

  it('does not seed while the OU is still loading', () => {
    const onOuDefaultsChange = vi.fn();
    mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: true});

    render(
      <ConfigureApplicationDetails
        {...baseProps}
        resolvedOuId="ou-1"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={onOuDefaultsChange}
      />,
    );

    expect(onOuDefaultsChange).not.toHaveBeenCalled();
  });

  it('does not re-seed (and so does not fight a manual uncheck) on a re-render for the same OU', () => {
    const onOuDefaultsChange = vi.fn();
    mockUseGetOrganizationUnit.mockReturnValue({data: buildOu(), isLoading: false});

    const {rerender} = render(
      <ConfigureApplicationDetails
        {...baseProps}
        resolvedOuId="ou-1"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={onOuDefaultsChange}
      />,
    );
    expect(onOuDefaultsChange).toHaveBeenCalledTimes(1);

    // Simulate the user manually unchecking sign-in after the initial seed, then a re-render
    // (e.g. from an unrelated state change) for the same OU.
    const afterManualUncheck: OrganizationUnitDefaultsSelection = {
      ...NO_DEFAULTS,
      [OrganizationUnitDefaultItem.SIGN_IN]: false,
      [OrganizationUnitDefaultItem.THEME]: true,
    };
    rerender(
      <ConfigureApplicationDetails
        {...baseProps}
        resolvedOuId="ou-1"
        ouDefaults={afterManualUncheck}
        onOuDefaultsChange={onOuDefaultsChange}
      />,
    );

    expect(onOuDefaultsChange).toHaveBeenCalledTimes(1);
  });

  it('re-seeds when the resolved OU changes (e.g. after "Change")', () => {
    const onOuDefaultsChange = vi.fn();
    mockUseGetOrganizationUnit.mockReturnValue({data: buildOu(), isLoading: false});

    const {rerender} = render(
      <ConfigureApplicationDetails
        {...baseProps}
        resolvedOuId="ou-1"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={onOuDefaultsChange}
      />,
    );
    expect(onOuDefaultsChange).toHaveBeenCalledTimes(1);

    mockUseGetOrganizationUnit.mockReturnValue({
      data: buildOu({id: 'ou-2', name: 'Other', authFlowId: undefined, layoutId: 'layout-2'}),
      isLoading: false,
    });
    rerender(
      <ConfigureApplicationDetails
        {...baseProps}
        resolvedOuId="ou-2"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={onOuDefaultsChange}
      />,
    );

    expect(onOuDefaultsChange).toHaveBeenCalledTimes(2);
    expect(onOuDefaultsChange).toHaveBeenLastCalledWith({
      ...NO_DEFAULTS,
      [OrganizationUnitDefaultItem.THEME]: true,
      [OrganizationUnitDefaultItem.LAYOUT]: true,
    });
  });

  it('renders the organization unit defaults section once resolved', () => {
    mockUseGetOrganizationUnit.mockReturnValue({data: buildOu(), isLoading: false});

    render(
      <ConfigureApplicationDetails
        {...baseProps}
        resolvedOuId="ou-1"
        ouDefaults={NO_DEFAULTS}
        onOuDefaultsChange={vi.fn()}
      />,
    );

    expect(screen.getByTestId('application-configure-ou-defaults')).toBeInTheDocument();
  });

  describe('User Access', () => {
    const userTypes = [
      {id: '1', name: 'Internal', ouId: 'INTERNAL', allowSelfRegistration: true},
      {id: '2', name: 'External', ouId: 'EXTERNAL', allowSelfRegistration: false},
    ];

    it('renders the user access section when two or more user types exist', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false});

      render(
        <ConfigureApplicationDetails
          {...baseProps}
          resolvedOuId={undefined}
          ouDefaults={NO_DEFAULTS}
          onOuDefaultsChange={vi.fn()}
          userTypes={userTypes}
        />,
      );

      expect(screen.getByTestId('application-configure-user-access')).toBeInTheDocument();
    });

    it('does not render or require user access when the application disallows user logins', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false});
      const onReadyChange = vi.fn();
      const onUserTypesChange = vi.fn();

      render(
        <ConfigureApplicationDetails
          {...baseProps}
          hasSecurityStep={false}
          allowsUserLogins={false}
          resolvedOuId={undefined}
          ouDefaults={NO_DEFAULTS}
          onOuDefaultsChange={vi.fn()}
          userTypes={userTypes}
          selectedUserTypes={[]}
          onUserTypesChange={onUserTypesChange}
          onReadyChange={onReadyChange}
        />,
      );

      expect(screen.queryByTestId('application-configure-user-access')).not.toBeInTheDocument();
      expect(onUserTypesChange).not.toHaveBeenCalled();
      expect(onReadyChange).toHaveBeenCalledWith(true);
    });

    it('uses the explicit user-login setting independently of the Security step', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false});

      render(
        <ConfigureApplicationDetails
          {...baseProps}
          hasSecurityStep={false}
          allowsUserLogins={true}
          resolvedOuId={undefined}
          ouDefaults={NO_DEFAULTS}
          onOuDefaultsChange={vi.fn()}
          userTypes={userTypes}
        />,
      );

      expect(screen.getByTestId('application-configure-user-access')).toBeInTheDocument();
    });

    it('does not render the user access section for a single user type', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false});

      render(
        <ConfigureApplicationDetails
          {...baseProps}
          resolvedOuId={undefined}
          ouDefaults={NO_DEFAULTS}
          onOuDefaultsChange={vi.fn()}
          userTypes={[userTypes[0]]}
        />,
      );

      expect(screen.queryByTestId('application-configure-user-access')).not.toBeInTheDocument();
    });

    it('defaults to selecting every user type once they load', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false});
      const onUserTypesChange = vi.fn();

      render(
        <ConfigureApplicationDetails
          {...baseProps}
          resolvedOuId={undefined}
          ouDefaults={NO_DEFAULTS}
          onOuDefaultsChange={vi.fn()}
          userTypes={userTypes}
          selectedUserTypes={[]}
          onUserTypesChange={onUserTypesChange}
        />,
      );

      expect(onUserTypesChange).toHaveBeenCalledWith(['Internal', 'External']);
    });

    it('reports the step as not ready when multiple user types exist and none are selected', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false});
      const onReadyChange = vi.fn();

      render(
        <ConfigureApplicationDetails
          {...baseProps}
          resolvedOuId={undefined}
          ouDefaults={NO_DEFAULTS}
          onOuDefaultsChange={vi.fn()}
          userTypes={userTypes}
          selectedUserTypes={[]}
          onUserTypesChange={vi.fn()}
          onReadyChange={onReadyChange}
        />,
      );

      expect(onReadyChange).toHaveBeenCalledWith(false);
    });

    it('reports the step as ready once a user type is selected', () => {
      mockUseGetOrganizationUnit.mockReturnValue({data: undefined, isLoading: false});
      const onReadyChange = vi.fn();

      render(
        <ConfigureApplicationDetails
          {...baseProps}
          appName="My App"
          resolvedOuId={undefined}
          ouDefaults={NO_DEFAULTS}
          onOuDefaultsChange={vi.fn()}
          userTypes={userTypes}
          selectedUserTypes={['Internal']}
          onUserTypesChange={vi.fn()}
          onReadyChange={onReadyChange}
        />,
      );

      expect(onReadyChange).toHaveBeenCalledWith(true);
    });
  });
});
