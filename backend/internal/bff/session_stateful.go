// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	// touchInterval bounds how often a session's idle timeout is pushed out, so a busy session does
	// not write to the store on every request.
	touchInterval = 60
	// refreshLockTTL bounds how long a refresh holds the session's refresh lock if it never releases it.
	refreshLockTTL = 15
	// recordVersionField is the record field a refresh compares before it replaces the record.
	recordVersionField = "v"
)

// sessionRecord is a session as the runtime store holds it. The tokens are encrypted, and the version
// changes on every refresh so concurrent refreshes cannot overwrite each other.
type sessionRecord struct {
	Version string `json:"v"`
	Data    string `json:"data"`
	Seen    int64  `json:"seen"`
}

// statefulSessions keeps sessions in the runtime store, shared by every replica, behind an opaque
// cookie. The store's TTL is the idle timeout: a session not used for that long is gone. An index from
// the SSO session ID to the record lets a logout remove the record without the browser's cookie.
type statefulSessions struct {
	store       providers.RuntimeStoreProvider
	sealer      sealer
	cookie      cookieSpec
	idleTimeout int64
	now         func() time.Time
	logger      *log.Logger

	// refreshWait and refreshPoll govern how long a request waits for a refresh another request runs.
	refreshWait time.Duration
	refreshPoll time.Duration
}

// recordKey derives a record's key from the cookie's handle. The store holds only the hash, so its
// contents cannot be replayed as cookies.
func recordKey(handle string) string {
	sum := sha256.Sum256([]byte(handle))
	return hex.EncodeToString(sum[:])
}

func (s *statefulSessions) carries(r *http.Request) bool {
	_, ok := s.cookie.read(r)
	return ok
}

func (s *statefulSessions) peek(r *http.Request) (*session, bool) {
	handle, ok := s.cookie.read(r)
	if !ok {
		return nil, false
	}
	_, sess, err := s.get(r, recordKey(handle))
	if err != nil || sess == nil {
		return nil, false
	}
	return sess, true
}

func (s *statefulSessions) load(w http.ResponseWriter, r *http.Request) (*session, bool) {
	ctx := r.Context()
	handle, ok := s.cookie.read(r)
	if !ok {
		return nil, false
	}
	key := recordKey(handle)
	record, sess, err := s.get(r, key)
	if err != nil || sess == nil {
		if err != nil {
			s.logger.Warn(ctx, "Failed to read a session", log.Error(err))
		}
		s.cookie.clear(w, r)
		return nil, false
	}

	if now := s.now().Unix(); now-record.Seen >= touchInterval {
		s.touch(r, key, sess.SessionID, record, now)
	}
	return sess, true
}

// get reads and opens a record. A missing record is a nil session and no error.
func (s *statefulSessions) get(r *http.Request, key string) (*sessionRecord, *session, error) {
	ctx := r.Context()
	value, err := s.store.Get(ctx, providers.NamespaceConsoleSession, key)
	if err != nil {
		if errors.Is(err, providers.ErrRuntimeStoreKeyNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if value == nil {
		return nil, nil, nil
	}

	var record sessionRecord
	if err := json.Unmarshal(value, &record); err != nil {
		return nil, nil, err
	}
	var sess session
	if err := s.sealer.open(ctx, record.Data, &sess); err != nil {
		return nil, nil, err
	}
	sess.key = key
	sess.version = record.Version
	return &record, &sess, nil
}

// touch pushes the idle timeout out and records when. A failure only means the session may expire a
// little early, so it is logged and otherwise ignored.
func (s *statefulSessions) touch(r *http.Request, key, sessionID string, record *sessionRecord, now int64) {
	ctx := r.Context()
	if err := s.store.ExtendTTL(ctx, providers.NamespaceConsoleSession, key, s.idleTimeout); err != nil {
		s.logger.Debug(ctx, "Failed to extend a session", log.Error(err))
		return
	}
	if sessionID != "" {
		if err := s.store.ExtendTTL(ctx, providers.NamespaceConsoleSessionByID, sessionID,
			s.idleTimeout); err != nil {
			s.logger.Debug(ctx, "Failed to extend a session's index entry", log.Error(err))
		}
	}
	touched := *record
	touched.Seen = now
	if value, err := json.Marshal(touched); err == nil {
		_, _ = s.store.CompareFieldAndSwap(ctx, providers.NamespaceConsoleSession, key, recordVersionField,
			record.Version, value)
	}
}

func (s *statefulSessions) save(w http.ResponseWriter, r *http.Request, sess *session) error {
	ctx := r.Context()
	if handle, ok := s.cookie.read(r); ok {
		_ = s.store.Delete(ctx, providers.NamespaceConsoleSession, recordKey(handle))
	}

	handle := randomToken()
	value, err := s.record(r, sess, randomToken())
	if err != nil {
		return err
	}
	key := recordKey(handle)
	if err := s.store.Put(ctx, providers.NamespaceConsoleSession, key, value, s.idleTimeout); err != nil {
		return err
	}
	// The index keeps the latest record for the SSO session. A logout still reaches an older record
	// through the ended session check, and its idle timeout removes it.
	if sess.SessionID != "" {
		if err := s.store.Put(ctx, providers.NamespaceConsoleSessionByID, sess.SessionID, []byte(key),
			s.idleTimeout); err != nil {
			_ = s.store.Delete(ctx, providers.NamespaceConsoleSession, key)
			return err
		}
	}
	s.cookie.write(w, r, handle, 0)
	return nil
}

// refresh renews the tokens once for every request and replica sharing the session. The request that
// takes the refresh lock runs the grant and replaces the record. Any other waits for the record to
// change and uses the result, because the authorization server rotates the refresh token and a second
// grant with the old one would fail.
func (s *statefulSessions) refresh(_ http.ResponseWriter, r *http.Request, sess *session,
	renew renewFunc) (*session, error) {
	ctx := r.Context()
	if sess.RefreshToken == "" {
		return nil, errNoRefreshToken
	}

	locked, err := s.store.PutIfNotExists(ctx, providers.NamespaceConsoleRefreshLock, sess.key, []byte("1"),
		refreshLockTTL)
	if err != nil {
		return nil, err
	}
	if !locked {
		return s.awaitRefresh(r, sess)
	}
	defer func() {
		_ = s.store.Delete(ctx, providers.NamespaceConsoleRefreshLock, sess.key)
	}()

	tokens, err := renew(ctx, sess.RefreshToken)
	if err != nil {
		return nil, err
	}
	renewed := sessionFromTokens(tokens, sess, s.now())
	renewed.version = randomToken()
	value, err := s.record(r, renewed, renewed.version)
	if err != nil {
		return nil, err
	}
	swapped, err := s.store.CompareFieldAndSwap(ctx, providers.NamespaceConsoleSession, sess.key,
		recordVersionField, sess.version, value)
	if err != nil {
		return nil, err
	}
	if !swapped {
		return nil, errSessionGone
	}
	return renewed, nil
}

// awaitRefresh waits for the refresh another request is running and returns its result.
func (s *statefulSessions) awaitRefresh(r *http.Request, sess *session) (*session, error) {
	ctx := r.Context()
	deadline := time.NewTimer(s.refreshWait)
	defer deadline.Stop()
	ticker := time.NewTicker(s.refreshPoll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, errors.New("timed out waiting for a concurrent refresh")
		case <-ticker.C:
			_, current, err := s.get(r, sess.key)
			if err != nil {
				return nil, err
			}
			if current == nil {
				return nil, errSessionGone
			}
			if current.version != sess.version {
				return current, nil
			}
		}
	}
}

func (s *statefulSessions) destroy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if handle, ok := s.cookie.read(r); ok {
		key := recordKey(handle)
		if _, sess, err := s.get(r, key); err == nil && sess != nil && sess.SessionID != "" {
			s.unindex(ctx, sess.SessionID, key)
		}
		if err := s.store.Delete(ctx, providers.NamespaceConsoleSession, key); err != nil {
			s.logger.Warn(ctx, "Failed to delete a session", log.Error(err))
		}
	}
	s.cookie.clear(w, r)
}

// unindex removes the SSO session's index entry if it still points at the record.
func (s *statefulSessions) unindex(ctx context.Context, sessionID, key string) {
	value, err := s.store.Get(ctx, providers.NamespaceConsoleSessionByID, sessionID)
	if err != nil || string(value) != key {
		return
	}
	if err := s.store.Delete(ctx, providers.NamespaceConsoleSessionByID, sessionID); err != nil {
		s.logger.Debug(ctx, "Failed to delete a session's index entry", log.Error(err))
	}
}

// forget deletes the record held for an SSO session that ended, so its tokens leave the store
// without waiting for the browser's next request.
func (s *statefulSessions) forget(ctx context.Context, sessionID string) error {
	value, err := s.store.Take(ctx, providers.NamespaceConsoleSessionByID, sessionID)
	if err != nil {
		if errors.Is(err, providers.ErrRuntimeStoreKeyNotFound) {
			return nil
		}
		return err
	}
	if len(value) == 0 {
		return nil
	}
	return s.store.Delete(ctx, providers.NamespaceConsoleSession, string(value))
}

func (s *statefulSessions) record(r *http.Request, sess *session, version string) ([]byte, error) {
	data, err := s.sealer.seal(r.Context(), sess)
	if err != nil {
		return nil, err
	}
	return json.Marshal(sessionRecord{Version: version, Data: data, Seen: s.now().Unix()})
}
