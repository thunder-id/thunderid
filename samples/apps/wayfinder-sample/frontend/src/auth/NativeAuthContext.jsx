/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { createContext, useContext, useState } from "react";

const STORAGE_KEY = "wf_native_token";

export const NativeAuthContext = createContext(null);

function decodeJWTPayload(token) {
  try {
    const b64 = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = b64 + "=".repeat((4 - (b64.length % 4)) % 4);
    return JSON.parse(atob(padded));
  } catch {
    return null;
  }
}

function isTokenExpired(token) {
  const payload = decodeJWTPayload(token);
  if (!payload?.exp) return false;
  return Date.now() / 1000 >= payload.exp;
}

export function NativeAuthProvider({ children }) {
  const [token, setTokenState] = useState(() => {
    const stored = sessionStorage.getItem(STORAGE_KEY);
    if (stored && isTokenExpired(stored)) {
      sessionStorage.removeItem(STORAGE_KEY);
      return null;
    }
    return stored;
  });

  const user = token ? decodeJWTPayload(token) : null;

  function setToken(accessToken) {
    sessionStorage.setItem(STORAGE_KEY, accessToken);
    setTokenState(accessToken);
  }

  function clearToken() {
    sessionStorage.removeItem(STORAGE_KEY);
    setTokenState(null);
  }

  async function getAccessToken() {
    if (!token) return null;
    if (isTokenExpired(token)) {
      clearToken();
      return null;
    }
    return token;
  }

  return (
    <NativeAuthContext.Provider
      value={{
        isSignedIn: Boolean(token),
        isLoading: false,
        user,
        token,
        setToken,
        clearToken,
        getAccessToken,
      }}
    >
      {children}
    </NativeAuthContext.Provider>
  );
}

export function useNativeAuth() {
  return useContext(NativeAuthContext);
}
