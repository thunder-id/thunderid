// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo, type PropsWithChildren, type ReactElement} from 'react';
import FlowBuilderCoreProvider from './FlowBuilderCoreProvider';
import ResourceProperties from '../components/resource-property-panel/ResourceProperties';
import ElementFactory from '../components/resources/elements/ElementFactory';
import {PreviewScreenType} from '../models/custom-text-preference';
/**
 * This component provides login flow builder related context to its children.
 *
 * @param props - Props injected to the component.
 * @returns The FlowBuilderProvider component.
 */
function FlowBuilderProvider({children}: PropsWithChildren): ReactElement {
  const screensList: PreviewScreenType[] = useMemo(
    () => [
      PreviewScreenType.SIGN_UP,
      PreviewScreenType.COMMON,
      PreviewScreenType.EMAIL_LINK_EXPIRY,
      PreviewScreenType.SMS_OTP,
      PreviewScreenType.EMAIL_OTP,
    ],
    [],
  );

  return (
    <FlowBuilderCoreProvider
      ElementFactory={ElementFactory}
      ResourceProperties={ResourceProperties}
      screenTypes={screensList}
    >
      {children}
    </FlowBuilderCoreProvider>
  );
}

export default FlowBuilderProvider;
