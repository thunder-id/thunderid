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

import {I18nDefaultConstants} from '@thunderid/i18n';
import {Stack, Typography} from '@wso2/oxygen-ui';
import {CogIcon} from '@wso2/oxygen-ui-icons-react';
import {type EdgeTypes, type Node, type NodeTypes, ReactFlowProvider} from '@xyflow/react';
import merge from 'lodash-es/merge';
import startCase from 'lodash-es/startCase';
import {
  type FunctionComponent,
  type PropsWithChildren,
  type ReactElement,
  type ReactNode,
  useCallback,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from 'react';
import FlowConfigContext from './FlowConfigContext';
import FlowEventsProvider from './FlowEventsProvider';
import FlowPluginProvider from './FlowPluginProvider';
import I18nContext from './I18nContext';
import InteractionContext from './InteractionContext';
import UIPanelContext from './UIPanelContext';
import type {ValidationConfig} from './ValidationContext';
import ValidationProvider from './ValidationProvider';
import {PreviewScreenType} from '../models/custom-text-preference';
import type {FlowCompletionConfigsInterface} from '../models/flows';
import type {Claim} from '../models/metadata';
import {type Resource, ResourceTypes} from '../models/resources';
import {StepTypes, EdgeStyleTypes, type EdgeStyleTypes as EdgeStyleTypesType} from '../models/steps';
import type {GraphValidationRule} from '../validation/validation-rules';

/**
 * Props interface for ElementFactory component
 */
export interface ElementFactoryProps {
  resource?: Resource;
  stepId: string;
  [key: string]: unknown;
}

/**
 * Props interface for ResourceProperties component
 */
export interface ResourcePropertiesProps {
  properties?: Record<string, unknown>;
  /**
   * The resource associated with the property.
   */
  resource: Resource;
  /**
   * The event handler for the property change. Applies immediately by default.
   * Pass `true` as the 4th argument for text/number inputs to enable 300ms debouncing.
   */
  onChange: (
    propertyKey: string,
    newValue: string | boolean | number | object,
    resource: Resource,
    debounce?: boolean,
  ) => void;
  /**
   * The event handler for the variant change.
   * @param variant - The variant of the element.
   * @param resource - Partial resource properties to override.
   */
  onVariantChange?: (variant: string, resource?: Partial<Resource>) => void;
}

/**
 * Props interface of {@link FlowBuilderCoreProvider}
 */
export interface FlowBuilderCoreProviderProps {
  /**
   * The factory for creating nodes.
   */
  ElementFactory: FunctionComponent<ElementFactoryProps>;
  /**
   * The factory for creating element properties.
   */
  ResourceProperties: FunctionComponent<ResourcePropertiesProps>;
  /**
   * Screen types for the i18n text.
   * First provided screen type will be used as the primary screen type.
   */
  screenTypes: PreviewScreenType[];
  /**
   * Validation configuration settings.
   */
  validationConfig?: ValidationConfig;
}

/**
 * Inner component that uses useReactFlow hook and provides the core context.
 */
function FlowContextWrapper({
  children = null,
  ElementFactory,
  ResourceProperties,
  screenTypes,
  validationConfig = {},
}: PropsWithChildren<FlowBuilderCoreProviderProps>): ReactElement {
  // ── UI Panel State ──
  const [isResourcePanelOpen, setIsResourcePanelOpen] = useState<boolean>(true);
  const [isResourcePropertiesPanelOpen, setIsOpenResourcePropertiesPanel] = useState<boolean>(false);
  const [isVersionHistoryPanelOpen, setIsVersionHistoryPanelOpen] = useState<boolean>(false);
  const [resourcePropertiesPanelHeading, setResourcePropertiesPanelHeading] = useState<ReactNode>(null);

  // ── Interaction State ──
  const [lastInteractedElementInternal, setLastInteractedElementInternal] = useState<Resource>();
  const [lastInteractedStepId, setLastInteractedStepId] = useState<string>('');
  const [selectedAttributes, setSelectedAttributes] = useState<Record<string, Claim[]>>({});

  // ── Flow Config State ──
  const [flowCompletionConfigs, setFlowCompletionConfigs] = useState<FlowCompletionConfigsInterface>({});
  const [flowNodeTypes, setFlowNodeTypes] = useState<NodeTypes>({});
  const [flowEdgeTypes, setFlowEdgeTypes] = useState<EdgeTypes>({});
  const [isVerboseMode, setIsVerboseMode] = useState<boolean>(true);
  const [edgeStyle, setEdgeStyle] = useState<EdgeStyleTypesType>(EdgeStyleTypes.SmoothStep);
  const [flowNodes, setFlowNodes] = useState<Node[]>([]);
  const [graphValidationRules, setGraphValidationRules] = useState<GraphValidationRule[]>([]);

  // ── I18n State ──
  const [language, setLanguage] = useState<string>(I18nDefaultConstants.FALLBACK_LANGUAGE);

  const setResourcePropertiesPanelHeadingRef = useRef(setResourcePropertiesPanelHeading);
  const setLastInteractedElementInternalRef = useRef(setLastInteractedElementInternal);
  const setIsOpenResourcePropertiesPanelRef = useRef<Dispatch<SetStateAction<boolean>>>(
    setIsOpenResourcePropertiesPanel,
  );
  const setLastInteractedStepIdRef = useRef(setLastInteractedStepId);

  // Ref to store the callback to close the validation panel (for mutual exclusion)
  const closeValidationPanelRef = useRef<(() => void) | null>(null);

  // Temp variables for data fetching and error handling.
  const flowMetadata = undefined;
  const textPreference = null;
  const fallbackTextPreference = null;
  const supportedLocales = undefined;
  const textPreferenceLoading = false;
  const fallbackTextPreferenceLoading = false;
  const customTextPreferenceMetaLoading = false;
  const isFlowMetadataLoading = false;
  const isI18nSubmitting = false;

  // TODO: Implement i18n key update logic
  const updateI18nKey = useCallback(
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    async (_screenType: string, _language: string, _i18nText: Record<string, string>): Promise<boolean> =>
      Promise.resolve(false),
    [],
  );

  /**
   * Memoized i18n text combining both text preference and fallback.
   */
  const i18nText: Partial<Record<PreviewScreenType, Record<string, string>>> = useMemo(() => {
    if (!textPreference || !fallbackTextPreference) {
      return {};
    }

    return merge({}, fallbackTextPreference, textPreference);
  }, [textPreference, fallbackTextPreference]);

  /**
   * Memoized primary i18n screen based on the screen types.
   */
  const primaryI18nScreen: PreviewScreenType = useMemo(
    () => screenTypes?.[0] || PreviewScreenType.COMMON,
    [screenTypes],
  );

  const setLastInteractedResource = useCallback((resource: Resource, openPanel = true): void => {
    // Use display.header if available, otherwise fall back to the type
    const headerText = resource?.display?.header ?? startCase(resource?.type?.toLowerCase());

    setResourcePropertiesPanelHeadingRef.current(
      <Stack direction="row" className="sub-title" gap={1} alignItems="center">
        <CogIcon />
        <Typography variant="h5">{headerText} Properties</Typography>
      </Stack>,
    );
    setLastInteractedElementInternalRef.current(resource);

    // If openPanel is false, don't open the properties panel
    if (!openPanel) {
      return;
    }

    // If the element is a step node, do not open the properties panel for now.
    // TODO: Figure out if there are properties for a step.
    if (
      (resource.category === ResourceTypes.Step && resource.type === StepTypes.View) ||
      resource.resourceType === ResourceTypes.Template ||
      resource.resourceType === ResourceTypes.Widget
    ) {
      setIsOpenResourcePropertiesPanelRef.current(false);

      return;
    }

    setIsOpenResourcePropertiesPanelRef.current(true);
  }, []);

  const setLastInteractedStepIdStable = useCallback((stepId: string): void => {
    setLastInteractedStepIdRef.current(stepId);
  }, []);

  const setIsOpenResourcePropertiesPanelStable = useCallback((isOpen: boolean): void => {
    if (isOpen) {
      // Close validation panel when opening resource properties panel (mutual exclusion)
      closeValidationPanelRef.current?.();
    }
    setIsOpenResourcePropertiesPanelRef.current(isOpen);
  }, []);

  /**
   * Registers a callback to close the validation panel.
   * Called by ValidationProvider to enable mutual exclusion between panels.
   */
  const registerCloseValidationPanel = useCallback((callback: () => void): void => {
    closeValidationPanelRef.current = callback;
  }, []);

  const onResourceDropOnCanvas = useCallback(
    (resource: Resource, stepId: string): void => {
      // Pass false to not open the properties panel when adding from resource panel
      setLastInteractedResource(resource, false);
      setLastInteractedStepIdStable(stepId);
    },
    [setLastInteractedResource, setLastInteractedStepIdStable],
  );

  /**
   * Function to check if a given i18n key is custom.
   */
  const isCustomI18nKey: (key: string, excludePrimaryScreen?: boolean) => boolean = useCallback(
    (key: string, excludePrimaryScreen = true): boolean =>
      fallbackTextPreference
        ? Object.keys(fallbackTextPreference).every(
            (screenType: string) =>
              (screenType === primaryI18nScreen && excludePrimaryScreen) ||
              !fallbackTextPreference[screenType as PreviewScreenType][key],
          )
        : false,
    [fallbackTextPreference, primaryI18nScreen],
  );

  // ── Domain-specific context values ──

  const uiPanelValue = useMemo(
    () => ({
      isResourcePanelOpen,
      isResourcePropertiesPanelOpen,
      isVersionHistoryPanelOpen,
      resourcePropertiesPanelHeading,
      setIsResourcePanelOpen,
      setIsOpenResourcePropertiesPanel: setIsOpenResourcePropertiesPanelStable,
      setIsVersionHistoryPanelOpen,
      setResourcePropertiesPanelHeading,
      registerCloseValidationPanel,
    }),
    [
      isResourcePanelOpen,
      isResourcePropertiesPanelOpen,
      isVersionHistoryPanelOpen,
      resourcePropertiesPanelHeading,
      setIsOpenResourcePropertiesPanelStable,
      registerCloseValidationPanel,
    ],
  );

  const interactionValue = useMemo(
    () => ({
      lastInteractedResource: lastInteractedElementInternal!,
      lastInteractedStepId,
      setLastInteractedResource,
      setLastInteractedStepId: setLastInteractedStepIdStable,
      onResourceDropOnCanvas,
      selectedAttributes,
      setSelectedAttributes,
    }),
    [
      lastInteractedElementInternal,
      lastInteractedStepId,
      setLastInteractedResource,
      setLastInteractedStepIdStable,
      onResourceDropOnCanvas,
      selectedAttributes,
    ],
  );

  const flowConfigValue = useMemo(
    () => ({
      ElementFactory,
      ResourceProperties,
      flowCompletionConfigs,
      setFlowCompletionConfigs,
      metadata: flowMetadata,
      isFlowMetadataLoading,
      isVerboseMode,
      setIsVerboseMode,
      edgeStyle,
      setEdgeStyle,
      flowNodeTypes,
      flowEdgeTypes,
      setFlowNodeTypes,
      setFlowEdgeTypes,
      addResourceToFlow: undefined as ((resource: Resource) => void) | undefined,
      publishFlow: undefined as (() => Promise<boolean>) | undefined,
      flowNodes,
      setFlowNodes,
      graphValidationRules,
      setGraphValidationRules,
    }),
    [
      ElementFactory,
      ResourceProperties,
      flowCompletionConfigs,
      flowMetadata,
      isFlowMetadataLoading,
      isVerboseMode,
      edgeStyle,
      flowNodeTypes,
      flowEdgeTypes,
      flowNodes,
      graphValidationRules,
    ],
  );

  const i18nValue = useMemo(
    () => ({
      primaryI18nScreen,
      i18nText,
      i18nTextLoading: textPreferenceLoading || fallbackTextPreferenceLoading || customTextPreferenceMetaLoading,
      language,
      setLanguage,
      updateI18nKey,
      isI18nSubmitting,
      isCustomI18nKey,
      supportedLocales,
    }),
    [
      primaryI18nScreen,
      i18nText,
      textPreferenceLoading,
      fallbackTextPreferenceLoading,
      customTextPreferenceMetaLoading,
      language,
      updateI18nKey,
      isI18nSubmitting,
      isCustomI18nKey,
      supportedLocales,
    ],
  );

  return (
    <FlowEventsProvider>
      <FlowPluginProvider>
        <FlowConfigContext.Provider value={flowConfigValue}>
          <I18nContext.Provider value={i18nValue}>
            <UIPanelContext.Provider value={uiPanelValue}>
              <InteractionContext.Provider value={interactionValue}>
                <ValidationProvider validationConfig={validationConfig}>{children}</ValidationProvider>
              </InteractionContext.Provider>
            </UIPanelContext.Provider>
          </I18nContext.Provider>
        </FlowConfigContext.Provider>
      </FlowPluginProvider>
    </FlowEventsProvider>
  );
}

/**
 * FlowBuilderCoreProvider component.
 * This component provides flow builder core related context to its children.
 * It wraps the internal component with ReactFlowProvider to enable useReactFlow hook usage.
 *
 * @param props - Props injected to the component.
 * @returns The FlowBuilderCoreProvider component.
 */
function FlowBuilderCoreProvider({
  children = null,
  ...props
}: PropsWithChildren<FlowBuilderCoreProviderProps>): ReactElement {
  return (
    <ReactFlowProvider>
      <FlowContextWrapper {...props}>{children}</FlowContextWrapper>
    </ReactFlowProvider>
  );
}

export default FlowBuilderCoreProvider;
