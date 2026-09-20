// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Classname Utilities
export {default as cn, setCnPrefix, getCnPrefix} from './classnames/cn';

// String Operations
export {default as generateRandomHumanReadableIdentifiers} from './string/generateRandomHumanReadableIdentifiers';
export {default as kebabCase} from './string/kebabCase';

// Path Operations
export {default as isAbsoluteUrl} from './path/isAbsoluteUrl';
export {default as isRelativeUrl} from './path/isRelativeUrl';

// Object Operations
export {default as isEmpty} from './object/isEmpty';
export {default as isEqual} from './object/isEqual';
export {default as isEqualIgnoringEmpty} from './object/isEqualIgnoringEmpty';
export {default as isPlainObject} from './object/isPlainObject';
export {default as merge} from './object/merge';

// Error Utilities
export {default as getErrorMessage} from './error/getErrorMessage';

// Template Pattern Utilities
export {default as isI18nTemplatePattern, I18N_PATTERN, I18N_KEY_PATTERN} from './template/isI18nTemplatePattern';
export {default as isMetaTemplatePattern, META_PATTERN, META_KEY_PATTERN} from './template/isMetaTemplatePattern';
export {default as containsMetaTemplate, replaceMetaTemplate} from './template/containsMetaTemplate';
export {
  default as parseTemplateLiteral,
  TEMPLATE_LITERAL_REGEX,
  FUNCTION_CALL_REGEX,
  TemplateLiteralType,
} from './template/parseTemplateLiteral';
export type {TemplateLiteralResult, TemplateLiteralHandlers} from './template/parseTemplateLiteral';

// Validation
export {EMAIL_REGEX} from './validation/emailRegex';

// Administration Flows
export {
  ADMINISTRATION_FLOW_TYPE,
  AdministrationFlowConfigKey,
  AdministrationFlowData,
  AdministrationFlowInput,
  FLOW_PAGE_SIZE,
  FlowExecutionFailure,
  FlowStatus,
  executeAdministrationFlow,
  findAdministrationFlowId,
  resolveAdministrationFlowHandle,
  runAdministrationFlow,
} from './flow/administrationFlow';
export type {
  AdministrationFlowConfig,
  AdministrationFlowConfigKeyValue,
  BasicFlowSummary,
  FlowExecutionError,
  FlowExecutionResponse,
  FlowI18nMessage,
  FlowListResponse,
  HttpLike,
  ServerConfigLayers,
} from './flow/administrationFlow';
