// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Components
export {default as CopyableField} from './CopyableField/CopyableField';
export type {CopyableFieldProps} from './CopyableField/CopyableField';
export {default as CspOriginHint} from './CspOriginHint/CspOriginHint';
export type {CspOriginHintProps} from './CspOriginHint/CspOriginHint';
export {resolveCspHint} from './CspOriginHint/resolveCspHint';
export type {CspResourceType, CspHint} from './CspOriginHint/resolveCspHint';
export {default as ExternalLinkConfirmDialog} from './ExternalLinkConfirm/ExternalLinkConfirmDialog';
export type {ExternalLinkConfirmDialogProps} from './ExternalLinkConfirm/ExternalLinkConfirmDialog';
export {default as useExternalLinkConfirmation} from './ExternalLinkConfirm/useExternalLinkConfirmation';
export type {ExternalLinkConfirmationState} from './ExternalLinkConfirm/useExternalLinkConfirmation';
export {default as Helmet} from './Helmet/Helmet';
export type {HelmetProps} from './Helmet/Helmet';
export {default as FullScreenCreationWizardLayout} from './FullScreenCreationWizardLayout/FullScreenCreationWizardLayout';
export type {FullScreenCreationWizardLayoutProps} from './FullScreenCreationWizardLayout/FullScreenCreationWizardLayout';
export {default as I18nTextInput} from './I18nTextInput/I18nTextInput';
export type {I18nTextInputLabels, I18nTextInputProps} from './I18nTextInput/I18nTextInput';
export {default as ExternalLink} from './ExternalLink/ExternalLink';
export type {ExternalLinkProps} from './ExternalLink/ExternalLink';
export {default as OrganizationUnitSummaryChip} from './OrganizationUnitSummaryChip/OrganizationUnitSummaryChip';
export type {OrganizationUnitSummaryChipProps} from './OrganizationUnitSummaryChip/OrganizationUnitSummaryChip';
export {default as PageLoader} from './PageLoader/PageLoader';
export {default as QueryErrorNotice} from './QueryErrorNotice/QueryErrorNotice';
export type {QueryErrorNoticeProps, QueryErrorMessageResolver} from './QueryErrorNotice/QueryErrorNotice';
export {default as StackblitzQuickstartCard} from './StackblitzQuickstartCard/StackblitzQuickstartCard';
export type {StackblitzQuickstartCardProps} from './StackblitzQuickstartCard/StackblitzQuickstartCard';
export {default as ToggleCard} from './ToggleCard/ToggleCard';
export type {ToggleCardProps} from './ToggleCard/ToggleCard';

/* -------------------------- ICONS -------------------------- */

export {default as GithubIcon} from './icons/logos/vendor/GithubIcon';
export {default as GoogleIcon} from './icons/logos/vendor/GoogleIcon';
export {default as HeidiIcon} from './icons/logos/vendor/HeidiIcon';
export {default as LissiIcon} from './icons/logos/vendor/LissiIcon';
export {default as StackblitzIcon} from './icons/logos/vendor/StackblitzIcon';
export {default as AppleIcon} from './icons/logos/vendor/AppleIcon';
export {default as AndroidLogo} from './icons/logos/vendor/AndroidLogo';
export {default as FlutterLogo} from './icons/logos/vendor/FlutterLogo';
export {default as ReactIcon} from './icons/logos/vendor/ReactIcon';
export {default as VueIcon} from './icons/logos/vendor/VueIcon';
export {default as NextjsIcon} from './icons/logos/vendor/NextjsIcon';
export {default as NuxtIcon} from './icons/logos/vendor/NuxtIcon';
export {default as ExpressIcon} from './icons/logos/vendor/ExpressIcon';
export {default as NodeIcon} from './icons/logos/vendor/NodeIcon';
export {default as JavaScriptIcon} from './icons/logos/vendor/JavaScriptIcon';
export {default as PythonLogo} from './icons/logos/vendor/PythonLogo';
export {default as LangChainLogo} from './icons/logos/vendor/LangChainLogo';
export {default as JsonLogo} from './icons/logos/vendor/JsonLogo';
export {default as JwtLogo} from './icons/logos/vendor/JwtLogo';

/* -------------------------- LAB -------------------------- */

export {default as BuilderFloatingPanel} from './lab/components/BuilderLayout/BuilderFloatingPanel';
export {default as BuilderLayout} from './lab/components/BuilderLayout/BuilderLayout';
export {default as BuilderPanelHeader} from './lab/components/BuilderLayout/BuilderPanelHeader';
export {default as BuilderStaticPanel} from './lab/components/BuilderLayout/BuilderStaticPanel';
export {default as EmojiPicker} from './lab/components/EmojiPicker/EmojiPicker';
export {default as CopyableId} from './lab/components/CopyableId';
export {default as Kbd} from './lab/components/Kbd';
export {default as generateIconSuggestions} from './lab/components/EmojiPicker/utils/generateIconSuggestions';
export {default as LogoPicker} from './lab/components/LogoPicker/LogoPicker';
export type {LogoPickerProps} from './lab/components/LogoPicker/LogoPicker';
export {default as NameSuggestion} from './lab/components/NameSuggestion';
export type {NameSuggestionProps} from './lab/components/NameSuggestion';
export {default as PageLoadingAnimation} from './lab/components/PageLoadingAnimation';
export {default as ResourceAvatar} from './lab/components/ResourceAvatar';
export {default as SettingsCard} from './lab/components/SettingsCard';
export {default as UnsavedChangesBar} from './lab/components/UnsavedChangesBar';

// Utils
export {default as getInitials} from './lab/utils/getInitials';
