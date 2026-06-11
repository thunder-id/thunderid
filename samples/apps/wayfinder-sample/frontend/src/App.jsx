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

import { useEffect, useRef, useState } from "react";
import { useThunderID } from "@thunderid/react";
import {
  ChevronDown,
  CircleUserRound,
  LogOut,
  MessageCircle,
  Plane,
  Send,
  ShieldCheck,
  UserCog,
  X
} from "lucide-react";
import { Link, Navigate, Route, Routes, useLocation, useNavigate, useParams } from "react-router-dom";
import { getLocations } from "./api";
import { BookingDetailsPageWithAuth } from "./pages/BookingDetailsPageWithAuth";
import { BookingsPageWithAuth } from "./pages/BookingsPageWithAuth";
import { BookingsUnavailable } from "./pages/BookingsUnavailable";
import { FlightDetailsPage } from "./pages/FlightDetailsPage";
import { getDisplayName, HomePage, SignedInHomePage } from "./pages/HomePage";
import { ProfilePage } from "./pages/ProfilePage";
import { PaymentPageWithAuth } from "./pages/PaymentPageWithAuth";
import { ResultsPage, ResultsPageWithAuth } from "./pages/ResultsPage";
import { buildResultsPath, readCriteria } from "./utils/routes";

const AGENT_CHAT_URL = import.meta.env.VITE_AGENT_CHAT_URL || "http://localhost:8790/chat";
const THUNDER_BASE_URL = import.meta.env.VITE_THUNDER_BASE_URL || "";

function createChatMessage(role, content) {
  return {
    id: `${role}-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    role,
    content
  };
}

function AuthenticatedHeader({ authReady }) {
  if (!authReady) {
    return <SignedOutHeader disabled />;
  }

  return <LiveAuthHeader />;
}

function PrimaryNav({ authReady }) {
  if (!authReady) {
    return <PublicPrimaryNav />;
  }

  return <LivePrimaryNav />;
}

function PublicPrimaryNav() {
  return (
    <nav className="header-nav" aria-label="Primary navigation">
      <a href="/flights#search">Search</a>
      <a href="/flights#deals">Deals</a>
      <a href="/flights#faq">FAQ</a>
    </nav>
  );
}

function LivePrimaryNav() {
  const { isSignedIn } = useThunderID();

  if (isSignedIn) {
    return <span aria-hidden="true" />;
  }

  return <PublicPrimaryNav />;
}

function LiveAuthHeader() {
  const { isSignedIn, isLoading, signIn, clearSession, user } = useThunderID();
  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false);
  const accountMenuRef = useRef(null);
  const email = user?.email || user?.mail || "";
  const displayName = getDisplayName(user) || user?.sub || "Traveler";
  const showEmailSub = email && email !== displayName;

  async function handleSignOut() {
    try {
      await clearSession();
    } finally {
      window.location.replace("/flights");
    }
  }

  useEffect(() => {
    function handlePointerDown(event) {
      if (accountMenuRef.current && !accountMenuRef.current.contains(event.target)) {
        setIsAccountMenuOpen(false);
      }
    }

    document.addEventListener("pointerdown", handlePointerDown);

    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
    };
  }, []);

  if (isSignedIn) {
    return (
      <div className="auth-cluster account-menu-wrap" ref={accountMenuRef}>
        <button
          className="user-chip"
          type="button"
          aria-expanded={isAccountMenuOpen}
          aria-haspopup="menu"
          onClick={() => setIsAccountMenuOpen((current) => !current)}
        >
          <CircleUserRound className="user-chip-avatar" size={28} />
          <span className="user-chip-text">
            <span className="user-chip-name">{displayName}</span>
            {showEmailSub && <span className="user-chip-email">{email}</span>}
          </span>
          <ChevronDown
            className={`user-chip-chevron ${isAccountMenuOpen ? "user-chip-chevron--open" : ""}`}
            size={18}
          />
        </button>
        {isAccountMenuOpen && (
          <div className="account-menu" role="menu">
            <Link className="account-menu-item" to="/bookings" role="menuitem">
              <CircleUserRound size={18} />
              <span>My Bookings</span>
            </Link>
            <Link className="account-menu-item" to="/profile" role="menuitem">
              <UserCog size={18} />
              <span>Profile</span>
            </Link>
            <button
              className="account-menu-item"
              type="button"
              role="menuitem"
              onClick={handleSignOut}
            >
              <LogOut size={18} />
              <span>Sign Out</span>
            </button>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="auth-cluster">
      <button
        className="primary-small"
        type="button"
        disabled={isLoading}
        onClick={() => signIn({ acr_values: "urn:thunder:auth:user" })}
      >
        Sign in
      </button>
    </div>
  );
}

function SignedOutHeader({ disabled }) {
  return (
    <div className="auth-cluster">
      <button className="primary-small" type="button" disabled={disabled}>
        Sign in
      </button>
    </div>
  );
}

function FooterLinks({ authReady }) {
  if (!authReady) {
    return <PublicFooterLinks />;
  }

  return <LiveFooterLinks />;
}

function PublicFooterLinks() {
  return (
    <nav className="footer-links" aria-label="Footer navigation">
      <a href="/flights#search">Search</a>
      <a href="/flights#deals">Deals</a>
      <a href="/flights#faq">FAQ</a>
    </nav>
  );
}

function LiveFooterLinks() {
  const { isSignedIn } = useThunderID();

  if (isSignedIn) {
    return null;
  }

  return <PublicFooterLinks />;
}

function SiteFooter({ authReady }) {
  return (
    <footer className="site-footer">
      <div>
        <Link className="brand footer-brand" to="/flights" aria-label="Wayfinder Travel home">
          <span className="brand-mark">
            <Plane size={22} />
          </span>
          <span>Wayfinder</span>
        </Link>
        <p>Modern travel booking flows, with secure sign-in built in.</p>
      </div>
      <FooterLinks authReady={authReady} />
    </footer>
  );
}

// Opens an OAuth authorize URL in a popup. The /agent-callback page posts the
// auth code back via window.postMessage — the ChatWidgetCore message listener
// picks it up and submits it to POST /chat/consent.
function openConsentPopup(authorizeUrl) {
  if (!authorizeUrl) {
    return null;
  }

  return window.open(
    authorizeUrl,
    "wayfinder-agent-consent",
    "width=520,height=720,popup=yes"
  );
}

// In-chat prompt that asks the user to confirm BEFORE the OAuth popup opens.
// Clicking "Authorize" is a direct user gesture so browsers render a real popup.
function ConsentRequestBubble({ message, onUpdate }) {
  const status = message.status || "pending";

  function handleAuthorize() {
    const popup = openConsentPopup(message.consent?.authorize_url);
    if (!popup) {
      onUpdate({ status: "error", errorReason: "Popup was blocked by the browser" });
      return;
    }
    onUpdate({ status: "awaiting" });
  }

  function handleCancel() {
    onUpdate({ status: "cancelled" });
  }

  return (
    <div className="chat-message chat-message--assistant chat-message--consent">
      <p style={{ margin: 0 }}>{message.content}</p>
      {message.consent?.scope && (
        <p style={{ margin: "6px 0 0", fontSize: "12px", color: "#666" }}>
          Scope requested: <code>{message.consent.scope}</code>
        </p>
      )}
      <div style={{ display: "flex", gap: 8, marginTop: 10 }}>
        {status === "pending" && (
          <>
            <button
              type="button"
              onClick={handleAuthorize}
              style={{
                flex: "0 0 auto",
                whiteSpace: "nowrap",
                background: "#1f6feb",
                color: "white",
                border: "none",
                borderRadius: 6,
                padding: "8px 16px",
                cursor: "pointer",
                fontWeight: 600
              }}
            >
              Authorize
            </button>
            <button
              type="button"
              onClick={handleCancel}
              style={{
                flex: "0 0 auto",
                whiteSpace: "nowrap",
                background: "transparent",
                border: "1px solid #ccc",
                borderRadius: 6,
                padding: "8px 16px",
                cursor: "pointer"
              }}
            >
              Not now
            </button>
          </>
        )}
        {status === "awaiting" && (
          <p style={{ margin: 0, fontStyle: "italic", color: "#666" }}>
            Waiting for sign in…
          </p>
        )}
        {status === "authorized" && (
          <p style={{ margin: 0, fontStyle: "italic", color: "#28a745" }}>
            Authorized — retrying…
          </p>
        )}
        {status === "cancelled" && (
          <p style={{ margin: 0, fontStyle: "italic", color: "#888" }}>
            Cancelled.
          </p>
        )}
        {status === "error" && (
          <p style={{ margin: 0, color: "#c00" }}>
            {message.errorReason || "Could not start authorization."}
          </p>
        )}
      </div>
    </div>
  );
}

function ChatWidget({ authReady }) {
  if (authReady) {
    return <LiveChatWidget />;
  }

  return <ChatWidgetCore />;
}

function LiveChatWidget() {
  const { getAccessToken, isSignedIn } = useThunderID();

  return <ChatWidgetCore getToken={isSignedIn ? getAccessToken : null} />;
}

function ChatWidgetCore({ getToken }) {
  const [isOpen, setIsOpen] = useState(false);
  const [messages, setMessages] = useState([
    createChatMessage("assistant", "Hi, I can help with travel questions and booking details.")
  ]);
  const [draft, setDraft] = useState("");
  const [isProcessing, setIsProcessing] = useState(false);
  const sessionIdRef = useRef(null);
  const pendingRetryRef = useRef(null);
  const getTokenRef = useRef(getToken);
  const messagesEndRef = useRef(null);

  getTokenRef.current = getToken;

  async function sendChatMessage(message, addToUI = true) {
    if (addToUI) {
      setMessages((current) => [...current, createChatMessage("user", message)]);
    }
    setIsProcessing(true);

    try {
      const headers = { "Content-Type": "application/json" };
      const tokenFn = getTokenRef.current;

      if (tokenFn) {
        try {
          const token = await tokenFn();

          if (token) {
            headers["Authorization"] = `Bearer ${token}`;
          }
        } catch {
          // Continue without token — server will reject if auth is required.
        }
      }

      const res = await fetch(AGENT_CHAT_URL, {
        method: "POST",
        headers,
        body: JSON.stringify({
          message,
          session_id: sessionIdRef.current,
        }),
      });

      const data = await res.json();

      if (data.session_id) {
        sessionIdRef.current = data.session_id;
      }

      if (data.type === "response") {
        setMessages((current) => [
          ...current,
          createChatMessage("assistant", data.message || ""),
        ]);
      } else if (data.type === "need_user_consent") {
        pendingRetryRef.current = message;
        setMessages((current) => [
          ...current,
          {
            id: `consent-${data.request_id}`,
            role: "assistant",
            kind: "consent_request",
            content: "This action needs your permission. Sign in to authorize the assistant to act on your behalf.",
            consent: {
              authorize_url: data.authorize_url,
              state: data.state,
              request_id: data.request_id,
              scope: data.scope,
            },
            status: "pending",
          },
        ]);
      } else if (data.error) {
        setMessages((current) => [
          ...current,
          createChatMessage("assistant", data.error),
        ]);
      }
    } catch {
      setMessages((current) => [
        ...current,
        createChatMessage("assistant", "Failed to reach the assistant. Please try again."),
      ]);
    } finally {
      setIsProcessing(false);
    }
  }

  // Listen for the OAuth callback from the consent popup. When the popup
  // completes sign-in, it posts the authorization code back via postMessage.
  // We submit it to POST /chat/consent and then auto-retry the pending message.
  useEffect(() => {
    async function handleWindowMessage(event) {
      if (event.origin !== window.location.origin) {
        return;
      }

      if (!event.data || event.data.type !== "wayfinder-agent-oauth") {
        return;
      }

      const currentSessionId = sessionIdRef.current;

      if (!currentSessionId) {
        return;
      }

      if (event.data.error) {
        setMessages((current) =>
          current.map((m) =>
            m.kind === "consent_request" && m.status === "awaiting"
              ? { ...m, status: "error", errorReason: event.data.errorDescription || event.data.error }
              : m
          )
        );
        return;
      }

      if (event.data.code) {
        try {
          const consentUrl = `${AGENT_CHAT_URL}/consent`;
          const res = await fetch(consentUrl, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              session_id: currentSessionId,
              code: event.data.code,
              state: event.data.state,
            }),
          });
          const data = await res.json();

          setMessages((current) =>
            current.map((m) =>
              m.kind === "consent_request" && m.status === "awaiting"
                ? { ...m, status: "authorized" }
                : m
            )
          );

          if (data.type === "consent_received" && pendingRetryRef.current) {
            const retryMessage = pendingRetryRef.current;
            pendingRetryRef.current = null;
            await sendChatMessage(retryMessage, false);
          }
        } catch (error) {
          console.error("Consent submission failed:", error);
          setMessages((current) =>
            current.map((m) =>
              m.kind === "consent_request" && m.status === "awaiting"
                ? { ...m, status: "error", errorReason: "Failed to complete authorization" }
                : m
            )
          );
        }
      }
    }

    window.addEventListener("message", handleWindowMessage);
    return () => window.removeEventListener("message", handleWindowMessage);
  }, []);

  useEffect(() => {
    if (isOpen) {
      messagesEndRef.current?.scrollIntoView({ block: "end", behavior: "smooth" });
    }
  }, [isOpen, messages]);

  function handleSubmit(event) {
    event.preventDefault();

    const message = draft.trim();

    if (!message || isProcessing) {
      return;
    }

    setDraft("");
    sendChatMessage(message);
  }

  return (
    <div className="chat-widget">
      {isOpen && (
        <section className="chat-panel" aria-label="AI travel assistant">
          <header className="chat-header">
            <div>
              <span className="chat-kicker">AI assistant</span>
              <h2>Wayfinder Concierge</h2>
            </div>
            <div className="chat-header-actions">
              <button
                className="chat-icon-button"
                type="button"
                aria-label="Close AI chat"
                onClick={() => setIsOpen(false)}
              >
                <X size={18} />
              </button>
            </div>
          </header>
          <div className="chat-messages" role="log" aria-live="polite">
            {messages.map((message) => {
              if (message.kind === "consent_request") {
                return (
                  <ConsentRequestBubble
                    key={message.id}
                    message={message}
                    onUpdate={(patch) => {
                      setMessages((current) =>
                        current.map((m) => (m.id === message.id ? { ...m, ...patch } : m))
                      );
                    }}
                  />
                );
              }
              return (
                <div className={`chat-message chat-message--${message.role}`} key={message.id}>
                  {message.content}
                </div>
              );
            })}
            {isProcessing && (
              <div className="chat-message chat-message--assistant chat-message--typing">
                Thinking...
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>
          <form className="chat-composer" onSubmit={handleSubmit}>
            <label className="chat-input-label">
              <span>Ask the travel assistant</span>
              <input
                value={draft}
                placeholder="Ask about flights or bookings"
                onChange={(event) => setDraft(event.target.value)}
              />
            </label>
            <button
              className="chat-send-button"
              type="submit"
              disabled={!draft.trim() || isProcessing}
              aria-label="Send message"
            >
              <Send size={18} />
            </button>
          </form>
        </section>
      )}

      <button
        className="chat-launcher"
        type="button"
        aria-label={isOpen ? "Close AI chat" : "Open AI chat"}
        aria-expanded={isOpen}
        onClick={() => setIsOpen((current) => !current)}
      >
        {isOpen ? <X size={22} /> : <MessageCircle size={24} />}
      </button>
    </div>
  );
}

function FlightDetailsRoute({ criteria }) {
  const { flightId = "" } = useParams();

  return <FlightDetailsPage criteria={criteria} flightId={flightId} />;
}

function PaymentRoute({ criteria }) {
  const { flightId = "" } = useParams();

  return <PaymentPageWithAuth criteria={criteria} flightId={flightId} />;
}

function BookingDetailsRoute() {
  const { bookingId = "" } = useParams();

  return <BookingDetailsPageWithAuth bookingId={bookingId} />;
}

function LandingRoute({ authReady, category, locations, onSearch }) {
  if (authReady) {
    return <SignedInHomePage category={category} locations={locations} onSearch={onSearch} />;
  }

  return <HomePage category={category} locations={locations} onSearch={onSearch} />;
}

function HomeRedirect() {
  const params = new URLSearchParams(window.location.search);
  if (params.has("code") || params.has("error")) {
    return null;
  }
  return <Navigate to="/flights" replace />;
}

// Receives the OAuth authorization code from Thunder after the chat agent has
// triggered an authorization-code flow in a popup. Posts the code (and state)
// back to the opener window (the chat widget) and closes itself.
function AgentCallbackRoute() {
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");
    const error = params.get("error");
    const errorDescription = params.get("error_description");
    if (window.opener) {
      window.opener.postMessage(
        {
          type: "wayfinder-agent-oauth",
          code: code || null,
          state: state || null,
          error: error || null,
          errorDescription: errorDescription || null
        },
        window.location.origin
      );
    }
    // Give the parent a moment to receive the message before tearing the popup down.
    const timer = window.setTimeout(() => window.close(), 100);
    return () => window.clearTimeout(timer);
  }, []);

  return (
    <main style={{
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      minHeight: "100vh",
      fontFamily: "sans-serif",
      color: "#555"
    }}>
      <p>Connecting your account to the assistant…</p>
    </main>
  );
}

// Deep-link entry point for agent sign-in. Triggers an OAuth authorize with
// acr_values=urn:thunder:auth:agent so the gate renders the Agent ID/Secret
// form directly instead of the standard user credentials screen. Used by the
// wayfinder-agent-demo skill and any bookmarkable "Sign in as Agent" link.
function AgentSignInRoute() {
  const { isSignedIn, signIn } = useThunderID();
  const navigate = useNavigate();
  const triggered = useRef(false);

  useEffect(() => {
    if (isSignedIn) {
      navigate("/flights", { replace: true });
      return;
    }
    if (triggered.current) {
      return;
    }
    triggered.current = true;
    signIn({ acr_values: "urn:thunder:auth:agent" });
  }, [isSignedIn, signIn, navigate]);

  return (
    <main style={{
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      minHeight: "60vh",
      color: "#555"
    }}>
      <p>Redirecting to agent sign in…</p>
    </main>
  );
}

function AppRoutes({ authReady, criteria, locations, onSearch }) {
  return (
    <Routes>
      <Route path="/" element={<HomeRedirect />} />
      <Route
        path="/flights"
        element={
          <LandingRoute
            authReady={authReady}
            category="flights"
            locations={locations}
            onSearch={onSearch}
          />
        }
      />
      <Route
        path="/hotels"
        element={
          <LandingRoute
            authReady={authReady}
            category="hotels"
            locations={locations}
            onSearch={onSearch}
          />
        }
      />
      <Route
        path="/trips"
        element={
          <LandingRoute
            authReady={authReady}
            category="trips"
            locations={locations}
            onSearch={onSearch}
          />
        }
      />
      <Route
        path="/results"
        element={
          authReady ? (
            <ResultsPageWithAuth criteria={criteria} locations={locations} onSearch={onSearch} />
          ) : (
            <ResultsPage criteria={criteria} locations={locations} onSearch={onSearch} />
          )
        }
      />
      <Route path="/flights/:flightId" element={<FlightDetailsRoute criteria={criteria} />} />
      <Route
        path="/payment/flight/:flightId"
        element={authReady ? <PaymentRoute criteria={criteria} /> : <BookingsUnavailable />}
      />
      <Route
        path="/bookings/:bookingId"
        element={authReady ? <BookingDetailsRoute /> : <BookingsUnavailable />}
      />
      <Route
        path="/bookings"
        element={authReady ? <BookingsPageWithAuth /> : <BookingsUnavailable />}
      />
      <Route
        path="/profile"
        element={authReady ? <ProfilePage /> : <BookingsUnavailable />}
      />
      <Route path="/agent-callback" element={<AgentCallbackRoute />} />
      <Route
        path="/signin-as-agent"
        element={authReady ? <AgentSignInRoute /> : <BookingsUnavailable />}
      />
      <Route path="*" element={<Navigate to="/flights" replace />} />
    </Routes>
  );
}

function App({ authReady }) {
  const location = useLocation();
  const navigate = useNavigate();
  const [locations, setLocations] = useState({
    flights: [],
    hotels: [],
    trips: []
  });

  useEffect(() => {
    let isCurrent = true;

    async function loadLocations() {
      try {
        const [flightLocations, hotelLocations, tripLocations] = await Promise.all([
          getLocations({ category: "flights" }),
          getLocations({ category: "hotels" }),
          getLocations({ category: "trips" })
        ]);

        if (isCurrent) {
          setLocations({
            flights: flightLocations,
            hotels: hotelLocations,
            trips: tripLocations
          });
        }
      } catch {
        if (isCurrent) {
          setLocations({
            flights: [
              { name: "Colombo", type: "city" },
              { name: "Singapore", type: "city" },
              { name: "Tokyo", type: "city" },
              { name: "London", type: "city" },
              { name: "Dubai", type: "city" }
            ],
            hotels: [
              { name: "Singapore Marina", type: "area" },
              { name: "Tokyo Shibuya", type: "area" },
              { name: "London Kings Cross", type: "area" }
            ],
            trips: [
              { name: "Singapore", type: "destination" },
              { name: "Tokyo", type: "destination" },
              { name: "Dubai", type: "destination" }
            ]
          });
        }
      }
    }

    loadLocations();

    return () => {
      isCurrent = false;
    };
  }, []);

  function handleSearch(searchParams) {
    navigate(buildResultsPath(searchParams));
  }

  const criteria = readCriteria(location.search);

  return (
    <div className="app-shell">
      <header className="site-header">
        <Link className="brand" to="/flights" aria-label="Wayfinder Travel home">
          <span className="brand-mark">
            <Plane size={22} />
          </span>
          <span>Wayfinder</span>
        </Link>
        <PrimaryNav authReady={authReady} />
        <AuthenticatedHeader authReady={authReady} />
      </header>

      {!authReady && (
        <div className="setup-banner" role="status">
          <ShieldCheck size={18} />
          Add `VITE_THUNDER_CLIENT_ID` and `VITE_THUNDER_BASE_URL` to enable live
          sign in and sign out.
        </div>
      )}

      <AppRoutes
        authReady={authReady}
        criteria={criteria}
        locations={locations}
        onSearch={handleSearch}
      />
      <ChatWidget authReady={authReady} />
      <SiteFooter authReady={authReady} />
    </div>
  );
}

export default App;
