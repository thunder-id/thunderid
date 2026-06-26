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

import { useThunderID } from "@thunderid/react";
import { useNavigate } from "react-router-dom";
import { AUTH_CONFIG } from "./config";
import { useNativeAuth } from "./NativeAuthContext";

export function useAuth() {
  const thunderCtx = useThunderID();
  const nativeCtx = useNativeAuth();
  const navigate = useNavigate();

  if (AUTH_CONFIG.isRedirectBased) {
    return {
      isSignedIn: thunderCtx.isSignedIn,
      isLoading: thunderCtx.isLoading,
      user: thunderCtx.user,
      signIn: () => thunderCtx.signIn({ acr_values: "urn:thunder:auth:user" }),
      signOut: async () => {
        try {
          await thunderCtx.clearSession();
        } catch {
          // ignore — always redirect regardless
        }
        window.location.replace("/flights");
      },
      getAccessToken: thunderCtx.getAccessToken,
    };
  }

  return {
    isSignedIn: nativeCtx?.isSignedIn ?? false,
    isLoading: nativeCtx?.isLoading ?? false,
    user: nativeCtx?.user ?? null,
    signIn: () => navigate("/signin"),
    signOut: () => {
      nativeCtx?.clearToken();
      window.location.replace("/flights");
    },
    getAccessToken: nativeCtx?.getAccessToken ?? (() => Promise.resolve(null)),
  };
}
