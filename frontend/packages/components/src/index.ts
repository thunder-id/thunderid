/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Components
export {default as Helmet} from './Helmet/Helmet';
export type {HelmetProps} from './Helmet/Helmet';
export {default as I18nTextInput} from './I18nTextInput/I18nTextInput';
export type {I18nTextInputLabels, I18nTextInputProps} from './I18nTextInput/I18nTextInput';
export {default as PageLoader} from './PageLoader/PageLoader';

/* -------------------------- LAB -------------------------- */

export {default as BuilderFloatingPanel} from './lab/components/BuilderLayout/BuilderFloatingPanel';
export {default as BuilderLayout} from './lab/components/BuilderLayout/BuilderLayout';
export {default as BuilderPanelHeader} from './lab/components/BuilderLayout/BuilderPanelHeader';
export {default as BuilderStaticPanel} from './lab/components/BuilderLayout/BuilderStaticPanel';
export {default as EmojiPicker} from './lab/components/EmojiPicker/EmojiPicker';
export {default as CopyableId} from './lab/components/CopyableId';
export {default as Kbd} from './lab/components/Kbd';
export {default as generateIconSuggestions} from './lab/components/EmojiPicker/utils/generateIconSuggestions';
export {default as PageLoadingAnimation} from './lab/components/PageLoadingAnimation';
export {default as ResourceAvatar} from './lab/components/ResourceAvatar';
export {default as ResourceLogoDialog} from './lab/components/ResourceLogoDialog';
export {default as SettingsCard} from './lab/components/SettingsCard';
export {default as UnsavedChangesBar} from './lab/components/UnsavedChangesBar';

// Utils
export {default as getInitials} from './lab/utils/getInitials';
