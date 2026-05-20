import { useEffect, useRef, useState } from "react";
import { useThunderID } from "@thunderid/react";
import {
  Bot,
  ChevronDown,
  CircleUserRound,
  LogOut,
  MessageCircle,
  Plane,
  Send,
  ShieldCheck,
  X
} from "lucide-react";
import { Link, Navigate, Route, Routes, useLocation, useNavigate, useParams } from "react-router-dom";
import { getLocations } from "./api";
import { AgentPortalPage } from "./pages/AgentPortalPage";
import { BookingDetailsPageWithAuth } from "./pages/BookingDetailsPageWithAuth";
import { BookingsPageWithAuth } from "./pages/BookingsPageWithAuth";
import { BookingsUnavailable } from "./pages/BookingsUnavailable";
import { FlightDetailsPage } from "./pages/FlightDetailsPage";
import { HomePage, SignedInHomePage } from "./pages/HomePage";
import { PaymentPageWithAuth } from "./pages/PaymentPageWithAuth";
import { ResultsPage, ResultsPageWithAuth } from "./pages/ResultsPage";
import { buildResultsPath, readCriteria } from "./utils/routes";

const AGENT_CHAT_URL = import.meta.env.VITE_AGENT_CHAT_URL || "ws://localhost:8790/chat";
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
  const { isSignedIn, isLoading, signIn, signOut, signUpUrl, user } = useThunderID();
  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false);
  const accountMenuRef = useRef(null);
  const firstName = user?.name?.givenName || "";
  const lastName = user?.name?.familyName || "";
  const fullName = `${firstName} ${lastName}`.trim();
  const email = user?.email || user?.mail || user?.username || user?.userName || "";
  const displayName = fullName || email || user?.sub || "Traveler";

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
            {fullName && email && <span className="user-chip-email">{email}</span>}
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
            <Link className="account-menu-item" to="/agent-portal" role="menuitem">
              <Bot size={18} />
              <span>Agent Portal</span>
            </Link>
            <button
              className="account-menu-item"
              type="button"
              role="menuitem"
              onClick={() => signOut()}
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
        className="text-button"
        type="button"
        disabled={isLoading}
        onClick={() => signIn({ acr_values: "urn:thunder:auth:user" })}
      >
        Sign in
      </button>
      {signUpUrl && (
        <a className="primary-small" href={signUpUrl}>
          Sign up
        </a>
      )}
    </div>
  );
}

function SignedOutHeader({ disabled }) {
  return (
    <div className="auth-cluster">
      <button className="text-button" type="button" disabled={disabled}>
        Sign in
      </button>
      <button className="primary-small" type="button" disabled={disabled}>
        Sign up
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
        <p>Modern travel booking flows, secured with ThunderID.</p>
      </div>
      <FooterLinks authReady={authReady} />
    </footer>
  );
}

// Opens an OAuth authorize URL in a popup, waits for the /agent-callback page
// to post the auth code back via window.postMessage, then forwards the code
// over the WebSocket so the ai-agent can exchange it for a user-context token.
function handleNeedUserConsent(payload, socket) {
  const authorizeUrl = payload.authorize_url;
  const expectedState = payload.state;
  const requestId = payload.request_id;

  if (!authorizeUrl || !requestId) {
    return;
  }

  const popup = window.open(
    authorizeUrl,
    "wayfinder-agent-consent",
    "width=520,height=720,popup=yes"
  );

  if (!popup) {
    socket.send(JSON.stringify({
      type: "user_consent_error",
      request_id: requestId,
      error: "popup_blocked"
    }));
    return;
  }

  let settled = false;

  const onMessage = (event) => {
    if (event.origin !== window.location.origin) return;
    const data = event.data;
    if (!data || data.type !== "wayfinder-agent-oauth") return;
    if (expectedState && data.state !== expectedState) return;
    settled = true;
    window.removeEventListener("message", onMessage);
    window.clearInterval(closedPoll);
    if (data.error) {
      socket.send(JSON.stringify({
        type: "user_consent_error",
        request_id: requestId,
        error: data.error,
        error_description: data.errorDescription
      }));
    } else if (data.code) {
      socket.send(JSON.stringify({
        type: "user_code",
        request_id: requestId,
        code: data.code,
        state: data.state
      }));
    }
  };

  window.addEventListener("message", onMessage);

  // If the user closes the popup before completing the flow, signal cancel.
  const closedPoll = window.setInterval(() => {
    if (popup.closed && !settled) {
      window.removeEventListener("message", onMessage);
      window.clearInterval(closedPoll);
      socket.send(JSON.stringify({
        type: "user_consent_error",
        request_id: requestId,
        error: "popup_closed"
      }));
    }
  }, 500);
}

// In-chat prompt that asks the user to confirm BEFORE the OAuth popup opens.
// Clicking "Authorize" inside this bubble is what actually triggers
// handleNeedUserConsent. Because the click is a direct user gesture, browsers
// honour the popup window features and render a real popup (not a tab).
function ConsentRequestBubble({ message, onUpdate, socket }) {
  const status = message.status || "pending";

  function handleAuthorize() {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      onUpdate({ status: "error", errorReason: "Connection to assistant lost" });
      return;
    }
    onUpdate({ status: "awaiting" });
    handleNeedUserConsent(message.consent, socket);
  }

  function handleCancel() {
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({
        type: "user_consent_error",
        request_id: message.consent.request_id,
        error: "user_cancelled"
      }));
    }
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

function ChatWidget() {
  const [isOpen, setIsOpen] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState("disconnected");
  const [messages, setMessages] = useState([
    createChatMessage("assistant", "Hi, I can help with travel questions and booking details.")
  ]);
  const [draft, setDraft] = useState("");
  const [isProcessing, setIsProcessing] = useState(false);
  const socketRef = useRef(null);
  const reconnectTimerRef = useRef(null);
  const messagesEndRef = useRef(null);

  useEffect(() => {
    if (!isOpen) {
      setIsProcessing(false);
      setConnectionStatus("disconnected");
      return undefined;
    }

    let isCurrent = true;
    let retryDelay = 700;

    function connect() {
      if (!isCurrent) {
        return;
      }

      setConnectionStatus("connecting");
      const socket = new WebSocket(AGENT_CHAT_URL);
      socketRef.current = socket;

      socket.addEventListener("open", () => {
        if (!isCurrent) {
          return;
        }

        retryDelay = 700;
        setConnectionStatus("connected");
      });

      socket.addEventListener("message", (event) => {
        if (!isCurrent) {
          return;
        }

        let payload;

        try {
          payload = JSON.parse(event.data);
        } catch {
          payload = { type: "message", message: event.data };
        }

        if (payload.type === "message" || payload.type === "response") {
          setMessages((current) => [
            ...current,
            createChatMessage("assistant", payload.message || "")
          ]);
          setIsProcessing(false);
        } else if (payload.type === "error") {
          setMessages((current) => [
            ...current,
            createChatMessage("assistant", payload.message || "I could not process that request.")
          ]);
          setIsProcessing(false);
        } else if (payload.type === "need_user_consent") {
          // Don't auto-open the popup. Browsers demote window.open() to a
          // background tab when it's not triggered by a user gesture, which
          // is why the OAuth screen previously opened in a full tab. Instead,
          // render an in-chat prompt with an "Authorize" button. The user's
          // click is a direct gesture, so window.open will produce a real
          // popup window.
          setMessages((current) => [
            ...current,
            {
              id: `consent-${payload.request_id}`,
              role: "assistant",
              kind: "consent_request",
              content: `This action needs your permission. Sign in to authorize the assistant to act on your behalf.`,
              consent: {
                authorize_url: payload.authorize_url,
                state: payload.state,
                request_id: payload.request_id,
                scope: payload.scope
              },
              status: "pending"
            }
          ]);
          // The agent is now waiting for the user — turn off the "typing" dots.
          setIsProcessing(false);
        }
      });

      socket.addEventListener("close", () => {
        if (!isCurrent) {
          return;
        }

        setConnectionStatus("disconnected");
        socketRef.current = null;
        reconnectTimerRef.current = window.setTimeout(connect, retryDelay);
        retryDelay = Math.min(retryDelay * 1.6, 4000);
      });

      socket.addEventListener("error", () => {
        if (!isCurrent) {
          return;
        }

        setConnectionStatus("disconnected");
      });
    }

    connect();

    return () => {
      isCurrent = false;
      window.clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;

      if (socketRef.current) {
        socketRef.current.close();
        socketRef.current = null;
      }
    };
  }, [isOpen]);

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

    setMessages((current) => [...current, createChatMessage("user", message)]);
    setDraft("");
    setIsProcessing(true);

    if (socketRef.current?.readyState === WebSocket.OPEN) {
      socketRef.current.send(JSON.stringify({ message }));
      return;
    }

    setMessages((current) => [
      ...current,
      createChatMessage("assistant", "The travel assistant is reconnecting. Try again in a moment.")
    ]);
    setIsProcessing(false);
  }

  return (
    <div className="chat-widget">
      {isOpen && (
        <section className="chat-panel" aria-label="AI travel assistant">
          <header className="chat-header">
            <div>
              <span className="chat-kicker">AI assistant</span>
              <h2>Wayfinder concierge</h2>
            </div>
            <div className="chat-header-actions">
              <span className={`chat-status chat-status--${connectionStatus}`}>
                {connectionStatus}
              </span>
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
                    socket={socketRef.current}
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
        path="/agent-portal"
        element={authReady ? <AgentPortalPage /> : <BookingsUnavailable />}
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
          ThunderID sign in, sign up, and sign out.
        </div>
      )}

      <AppRoutes
        authReady={authReady}
        criteria={criteria}
        locations={locations}
        onSearch={handleSearch}
      />
      <ChatWidget />
      <SiteFooter authReady={authReady} />
    </div>
  );
}

export default App;
