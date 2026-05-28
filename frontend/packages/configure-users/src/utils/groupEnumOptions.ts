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

const DISPLAY_NAMES: Record<string, string> = {
  claude: 'Claude',
  'claude-opus-4.7': 'Opus 4.7',
  'claude-opus-4.6': 'Opus 4.6',
  'claude-sonnet-4.6': 'Sonnet 4.6',
  'claude-sonnet-4.5': 'Sonnet 4.5',
  'claude-haiku-4.5': 'Haiku 4.5',
  openai: 'OpenAI',
  'openai-gpt-5.4-pro': 'GPT-5.4 Pro',
  'openai-gpt-5.4-thinking': 'GPT-5.4 Thinking',
  'openai-gpt-5.4-mini': 'GPT-5.4 Mini',
  'openai-gpt-5.4-nano': 'GPT-5.4 Nano',
  'openai-gpt-5.3-instant': 'GPT-5.3 Instant',
  gemini: 'Gemini',
  'gemini-3.5-flash': '3.5 Flash',
  'gemini-3.1-pro': '3.1 Pro',
  'gemini-3-pro': '3 Pro',
  'gemini-3-flash': '3 Flash',
  llama: 'Llama',
  'llama-4-scout': '4 Scout',
  'llama-4-maverick': '4 Maverick',
  'llama-3.3-70b': '3.3 70B',
  mistral: 'Mistral',
  'mistral-large-3': 'Large 3',
  'mistral-small-4': 'Small 4',
  'mistral-medium-3.5': 'Medium 3.5',
  'mistral-devstral-2': 'Devstral 2',
  other: 'Other',
};

export const groupEnumOptions = (enumValues: string[]): Map<string, string[]> => {
  const groups = new Map<string, string[]>();

  for (const value of enumValues) {
    const dashIndex = value.indexOf('-');
    const provider = dashIndex > 0 ? value.substring(0, dashIndex) : value;

    const existing = groups.get(provider);
    if (existing) {
      existing.push(value);
    } else {
      groups.set(provider, [value]);
    }
  }

  return groups;
};

export const getModelDisplayName = (value: string): string => {
  if (DISPLAY_NAMES[value]) {
    return DISPLAY_NAMES[value];
  }

  const dashIndex = value.indexOf('-');
  if (dashIndex > 0) {
    const afterPrefix = value.substring(dashIndex + 1);
    return afterPrefix.charAt(0).toUpperCase() + afterPrefix.slice(1);
  }

  return value.charAt(0).toUpperCase() + value.slice(1);
};
