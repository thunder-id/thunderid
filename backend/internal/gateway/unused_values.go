// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/varname"
	"github.com/thunder-id/thunderid/internal/variablestore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// unusedValueSweeperInterface removes from a gateway's store the variables and secrets this plane
// named for a resource that no longer exists and that nothing the gateway runs, or could still be
// given, refers to any more.
//
// A resource removed from the configuration leaves its values behind on every gateway it was deployed
// to, and nothing else removes them, so without this a busy deployment's stores only grow.
type unusedValueSweeperInterface interface {
	// afterApply starts a sweep of the gateway's store once an apply recorded what the gateway holds,
	// unless one for the gateway is running or finished within the sweep interval. It returns at once:
	// the sweep runs on a context of its own, bounded by sweepTimeout, and a failure is logged, never
	// reported to the apply.
	afterApply(ctx context.Context, gw *Gateway, key string)
	StopperInterface
}

// StopperInterface stops the background work the gateway package starts. Only the composition root
// holds it.
type StopperInterface interface {
	// Stop refuses to start any more sweeps of a gateway's store, cancels those running, and waits for
	// them to end, for at most a fixed time. A second call returns at once.
	Stop()
}

// noUnusedValueSweep leaves every value in place, for a deployment that turned the removal off.
type noUnusedValueSweep struct{}

func (noUnusedValueSweep) afterApply(context.Context, *Gateway, string) {}

func (noUnusedValueSweep) Stop() {}

// valueStoreClient lists and removes the values a gateway's store holds, presenting its key.
type valueStoreClient interface {
	ListVariables(ctx context.Context, gw *Gateway, key string,
		query StoreListQuery) (*variablestore.VariableListResponse, error)
	ListSecrets(ctx context.Context, gw *Gateway, key string,
		query StoreListQuery) (*variablestore.SecretListResponse, error)
	// DeleteVariables and DeleteSecrets remove a batch nameBatches makes in one call and return
	// those the gateway removed. A positive unchangedFor removes only the values last set longer ago
	// than it, by the gateway's own clock, when they are removed; a younger one is kept and absent from
	// the answer.
	DeleteVariables(ctx context.Context, gw *Gateway, key string, names []string,
		unchangedFor time.Duration) ([]string, error)
	DeleteSecrets(ctx context.Context, gw *Gateway, key string, names []string,
		unchangedFor time.Duration) ([]string, error)
}

// sweepRunner runs a sweep apart from the apply that started it, so the apply answers without waiting.
type sweepRunner interface {
	run(sweep func())
}

// goroutineRunner runs each sweep on a goroutine of its own.
type goroutineRunner struct{}

func (goroutineRunner) run(sweep func()) { go sweep() }

// configurationExporter exports the configuration as it stands, the way a capture reads it, without
// keeping a version of it.
type configurationExporter interface {
	exportConfiguration(ctx context.Context) (*export.ExportResponse, *tidcommon.ServiceError)
}

// sweepTimeout bounds one sweep of a gateway's store. What a sweep did not reach is left for the
// sweep after the next apply.
const sweepTimeout = 10 * time.Minute

// sweepStopTimeout bounds how long Stop waits for the sweeps it cancelled to end.
const sweepStopTimeout = 10 * time.Second

// maxStorePages bounds how many pages of one collection a sweep reads, so a store that never says it
// is done cannot keep one going.
const maxStorePages = 10000

// maxCachedReferenceSets bounds how many versions' and masters' references are kept in memory. Past
// it the cache starts again: it only saves reading a version again.
const maxCachedReferenceSets = 512

// errNoCurrentConfiguration stops a sweep whose export of the configuration as it stands failed or
// came back without everything in it: a value of a resource left out would otherwise look unused.
var errNoCurrentConfiguration = errors.New("the current configuration could not be read in full")

// errNoRetainedVersion stops a sweep that found no version at all, which after an apply that recorded
// one means the versions could not be read as they are: every value would otherwise look unused.
var errNoRetainedVersion = errors.New("no retained configuration version was found")

// heldValue is a value a gateway's store holds, with when it was last set; zero when the store did
// not say.
type heldValue struct {
	name      string
	secret    bool
	updatedAt time.Time
}

// sweepOutcome counts what one sweep found and did.
type sweepOutcome struct {
	// listed is every value the store holds; generated those this plane named.
	listed, generated int
	// recent are the generated values set within the grace window, which stay whatever refers to them.
	recent int
	// removedVariables and removedSecrets are what the sweep removed.
	removedVariables, removedSecrets int
}

// unusedValueSweeper sweeps a gateway's store after each apply. A value is removed only when all of
// these hold:
//
//  1. Its name is one this plane generates for a resource's value. A name chosen by hand stays.
//  2. The configuration as it stands does not refer to it: a resource that still exists keeps its
//     values, whether or not it was ever deployed and however long its deploy waits. Deleting the
//     resource is what lets them go.
//  3. No version the deployment keeps refers to it, so any version a gateway can still be given,
//     rolled back to or promoted keeps its values. A version past gateway.max_versions is gone and
//     cannot be applied again.
//  4. No master the gateway keeps refers to it: what it runs now, and what a revert or a project's
//     rollback would return it to.
//  5. It was last set longer ago than the grace window, so a value set for a resource while a sweep
//     runs, before the resource is written, stays. The gateway checks the age itself, on its own
//     clock, in the statement that removes the value, and the age asked for adds the time since the
//     store was listed, so a value set again after the listing stays too, whatever this plane's
//     clock says, unless the request that removes it is delayed on its way to the gateway by more
//     than about the grace (see age). A gateway that cannot check it, one that predates the removal of many
//     values, has nothing removed.
//
// The configuration is exported once per sweep, which on a large deployment takes a while, so a
// gateway is swept at most once per interval: an apply within it starts none, and the next apply
// after it sweeps what the skipped ones would have.
type unusedValueSweeper struct {
	versions versionStoreInterface
	current  configurationExporter
	store    valueStoreClient
	grace    time.Duration
	// interval is the least time between the end of one sweep of a gateway and the start of the next,
	// in this process: each control plane node keeps its own, and a restart forgets it.
	interval time.Duration
	runner   sweepRunner
	// references caches the names each version and master refers to. Both are immutable once kept, so
	// each is read once: a version by its hash, which is the hash of its content, and a master by its
	// gateway, number and hash.
	references *referenceCache
	// mu guards gateways, where each gateway swept in this process has its sweep state, and stopped.
	mu       sync.Mutex
	gateways map[string]*gatewaySweep
	// stopped is set by Stop, after which no sweep starts.
	stopped bool
	// stopping is cancelled by Stop, which cancels every running sweep with it.
	stopping context.Context
	stop     context.CancelFunc
	// sweeps counts the running sweeps, so Stop can wait for them.
	sweeps sync.WaitGroup
	logger *log.Logger
}

// gatewaySweep is where the sweeps of one gateway stand in this process.
type gatewaySweep struct {
	// running is whether a sweep of the gateway is running, so applies in quick succession start one.
	running bool
	// finished is when the last sweep of the gateway ended; zero when none has.
	finished time.Time
	// toldNoBulkDelete is whether the gateway was found unable to remove values by age, which is said
	// once at warning level and after that only at debug level.
	toldNoBulkDelete bool
}

func newUnusedValueSweeper(versions versionStoreInterface, current configurationExporter,
	store valueStoreClient, runner sweepRunner, grace, interval time.Duration) *unusedValueSweeper {
	stopping, stop := context.WithCancel(context.Background())
	return &unusedValueSweeper{
		stopping:   stopping,
		stop:       stop,
		versions:   versions,
		current:    current,
		store:      store,
		runner:     runner,
		grace:      grace,
		interval:   interval,
		references: &referenceCache{sets: map[string]map[string]struct{}{}},
		gateways:   map[string]*gatewaySweep{},
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayUnusedValueSweeper")),
	}
}

func (s *unusedValueSweeper) afterApply(ctx context.Context, gw *Gateway, key string) {
	if reason := s.start(gw.ID); reason != "" {
		s.logger.Debug(ctx, reason, log.String("gatewayID", gw.ID))
		return
	}
	target := *gw
	// On a context of its own: the apply's request is answered before the sweep ends, and its context
	// goes with it. The deployment the request was for is kept. Stop cancels it.
	sweepCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sweepTimeout)
	stopCancelling := context.AfterFunc(s.stopping, cancel)
	s.runner.run(func() {
		// When it ended, not when it started: the interval is the rest the gateway gets between sweeps.
		defer s.finish(target.ID)
		defer stopCancelling()
		defer cancel()
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error(sweepCtx, "A sweep of the gateway's store stopped unexpectedly",
					log.String("gatewayID", target.ID), log.String("cause", fmt.Sprint(recovered)))
			}
		}()
		outcome, err := s.sweep(sweepCtx, &target, key)
		if err != nil {
			s.logger.Warn(sweepCtx, "Failed to remove unused values from the gateway's store",
				log.String("gatewayID", target.ID), log.Int("removedVariables", outcome.removedVariables),
				log.Int("removedSecrets", outcome.removedSecrets), log.Error(err))
			return
		}
		s.logger.Info(sweepCtx, "Swept the gateway's store for unused values", log.String("gatewayID", target.ID),
			log.Int("listed", outcome.listed), log.Int("generated", outcome.generated),
			log.Int("withinGrace", outcome.recent), log.Int("removedVariables", outcome.removedVariables),
			log.Int("removedSecrets", outcome.removedSecrets))
	})
}

// start marks a sweep of the gateway as running and answers empty, or answers why none starts: the
// sweeper was stopped, one is running, or one ended within the interval.
func (s *unusedValueSweeper) start(gatewayID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return "The removal of unused values was stopped"
	}
	state, ok := s.gateways[gatewayID]
	if !ok {
		state = &gatewaySweep{}
		s.gateways[gatewayID] = state
	}
	if state.running {
		return "A sweep of the gateway's store is already running"
	}
	if s.interval > 0 && !state.finished.IsZero() && time.Since(state.finished) < s.interval {
		return "The gateway's store was swept within the sweep interval"
	}
	state.running = true
	// Under the lock that Stop takes to set stopped, so no sweep is counted after Stop waits.
	s.sweeps.Add(1)
	return ""
}

// finish records that the gateway's sweep ended, now.
func (s *unusedValueSweeper) finish(gatewayID string) {
	defer s.sweeps.Done()
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.gateways[gatewayID]
	state.running = false
	state.finished = time.Now()
}

// Stop refuses to start any more sweeps, cancels those running, and waits up to sweepStopTimeout for
// them to end. A sweep cancelled part way leaves what it did not reach for a later one.
func (s *unusedValueSweeper) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()
	s.stop()

	ended := make(chan struct{})
	go func() {
		s.sweeps.Wait()
		close(ended)
	}()
	timer := time.NewTimer(sweepStopTimeout)
	defer timer.Stop()
	select {
	case <-ended:
	case <-timer.C:
		s.logger.Warn(context.Background(), "Sweeps of gateways' stores did not end in time on shutdown")
	}
}

// tellNoBulkDelete logs that the gateway cannot remove values by age: at warning level the first time
// for the gateway in this process, at debug level after that.
func (s *unusedValueSweeper) tellNoBulkDelete(ctx context.Context, gatewayID string) {
	s.mu.Lock()
	state := s.gateways[gatewayID]
	first := state != nil && !state.toldNoBulkDelete
	if state != nil {
		state.toldNoBulkDelete = true
	}
	s.mu.Unlock()
	const message = "The gateway's store cannot remove values by age, so no unused value was removed from it; " +
		"upgrade the gateway to have them removed"
	if first {
		s.logger.Warn(ctx, message, log.String("gatewayID", gatewayID))
		return
	}
	s.logger.Debug(ctx, message, log.String("gatewayID", gatewayID))
}

// sweep removes the gateway's unused values and reports what it found and did. The store is read
// before the versions and masters, so a version kept while the sweep runs is read with them.
func (s *unusedValueSweeper) sweep(ctx context.Context, gw *Gateway, key string) (sweepOutcome, error) {
	var outcome sweepOutcome
	// Taken before the store is asked, so any value set again after the listing was set after it.
	listedAt := time.Now()
	held, err := s.held(ctx, gw, key)
	if err != nil {
		return outcome, err
	}
	outcome.listed = len(held)
	// The store's own time is compared with this plane's clock only here, to leave out what is
	// plainly too recent. What decides is the gateway's own check of the age when it removes a value.
	cutoff := time.Now().UTC().Add(-s.grace)
	candidates := make([]heldValue, 0, len(held))
	for _, value := range held {
		// Only a name this plane generates for a resource's value, which the export writes as a
		// reference, is ever removed: a name an administrator chose is theirs.
		if _, generated := varname.IDSegmentOf(value.name); !generated {
			continue
		}
		outcome.generated++
		// A store that did not say when a value was set gives no way to tell it is old enough.
		if value.updatedAt.IsZero() || value.updatedAt.After(cutoff) {
			outcome.recent++
			continue
		}
		candidates = append(candidates, value)
	}
	if len(candidates) == 0 {
		return outcome, nil
	}
	kept, err := s.referenced(ctx, gw.ID)
	if err != nil {
		return outcome, err
	}
	var variables, secrets []string
	leftOutProtected := 0
	for _, value := range candidates {
		if _, ok := kept.names[value.name]; ok {
			continue
		}
		if kept.ofLeftOutResource(value.name) {
			leftOutProtected++
			continue
		}
		if value.secret {
			secrets = append(secrets, value.name)
		} else {
			variables = append(variables, value.name)
		}
	}
	if kept.leftOut > 0 {
		s.logger.Info(ctx, "The export of the current configuration left out resources; their values are kept",
			log.String("gatewayID", gw.ID), log.Int("leftOutResources", kept.leftOut),
			log.Int("valuesKept", leftOutProtected))
	}
	removal := &batchRemoval{sweeper: s, gw: gw, key: key, listedAt: listedAt}
	if outcome.removedVariables, err = removal.removeAll(ctx, variables, false); err != nil {
		return outcome, removal.unsupported(ctx, err)
	}
	outcome.removedSecrets, err = removal.removeAll(ctx, secrets, true)
	return outcome, removal.unsupported(ctx, err)
}

// batchRemoval removes one sweep's unused values from a gateway's store, as many in one call as
// nameBatches puts together, each only when the gateway finds it last set longer ago than the grace.
type batchRemoval struct {
	sweeper *unusedValueSweeper
	gw      *Gateway
	key     string
	// listedAt is when the sweep started to list the store, on this plane's monotonic clock.
	listedAt time.Time
}

// age is the age a value must have, when the gateway removes it, to be removed: the grace, and the
// time since the listing started rounded up to the second, and one second more for the whole seconds
// the gateway keeps. A value set at or after the listing is younger than that on the gateway's clock
// when the request is sent, so it is kept: only durations are carried from one clock to the other, so
// the two clocks need not agree. What this does not cover is the request itself: one that takes longer
// than the grace to reach the gateway's database, after a value nothing refers to was set again just
// after the listing, could still remove that value. Closing that would need a compare on each value's
// exact last-set time, round-tripped through both databases, which is not worth its risk for a value
// nothing refers to and a request stalled past the grace.
func (r *batchRemoval) age() time.Duration {
	elapsed := time.Since(r.listedAt)
	elapsed = (elapsed + time.Second - 1) / time.Second * time.Second
	return r.sweeper.grace + elapsed + time.Second
}

// removeAll removes the named variables, or secrets, and reports how many it removed.
func (r *batchRemoval) removeAll(ctx context.Context, names []string, secret bool) (int, error) {
	removed := 0
	for _, batch := range nameBatches(names) {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		gone, err := r.removeBatch(ctx, batch, secret)
		if err != nil {
			return removed, err
		}
		removed += len(gone)
		r.logRemoved(ctx, gone, secret)
	}
	return removed, nil
}

// removeBatch asks the gateway to remove the names last set longer ago than age. The gateway measures
// the age on its own clock, in the statement that removes them, so this plane's clock plays no part,
// and a value set again since the listing is kept unless this request is delayed by more than about
// the grace on its way to the gateway (see age).
func (r *batchRemoval) removeBatch(ctx context.Context, names []string, secret bool) ([]string, error) {
	age := r.age()
	if secret {
		return r.sweeper.store.DeleteSecrets(ctx, r.gw, r.key, names, age)
	}
	return r.sweeper.store.DeleteVariables(ctx, r.gw, r.key, names, age)
}

// unsupported answers the error a removal ended with, except that a gateway which does not serve the
// removal of many values by age is not a failure: it predates it, has no way to keep a value set
// again since the listing, and so has nothing removed. The first batch is refused, so nothing was.
func (r *batchRemoval) unsupported(ctx context.Context, err error) error {
	var refusal *StoreRefusal
	if errors.As(err, &refusal) &&
		(refusal.Status == http.StatusNotFound || refusal.Status == http.StatusMethodNotAllowed) {
		r.sweeper.tellNoBulkDelete(ctx, r.gw.ID)
		return nil
	}
	return err
}

func (r *batchRemoval) logRemoved(ctx context.Context, names []string, secret bool) {
	for _, name := range names {
		r.sweeper.logger.Debug(ctx, "Removed an unused value from the gateway's store",
			log.String("gatewayID", r.gw.ID), log.String("name", name), log.Bool("secret", secret))
	}
}

// held lists every variable and secret the gateway's store holds, a page at a time. The whole store
// is read before anything is removed, so a removal does not move the pages still to be read.
func (s *unusedValueSweeper) held(ctx context.Context, gw *Gateway, key string) ([]heldValue, error) {
	var held []heldValue
	for page, offset := 0, 0; page < maxStorePages; page++ {
		listing, err := s.store.ListVariables(ctx, gw, key, pageAt(offset))
		if err != nil {
			return nil, fmt.Errorf("failed to list the gateway's variables: %w", err)
		}
		for _, variable := range listing.Variables {
			held = append(held, heldValue{name: variable.Name, updatedAt: asTime(variable.UpdatedAt)})
		}
		offset += len(listing.Variables)
		if len(listing.Variables) == 0 || offset >= listing.TotalResults {
			break
		}
	}
	for page, offset := 0, 0; page < maxStorePages; page++ {
		listing, err := s.store.ListSecrets(ctx, gw, key, pageAt(offset))
		if err != nil {
			return nil, fmt.Errorf("failed to list the gateway's secrets: %w", err)
		}
		for _, secret := range listing.Secrets {
			held = append(held, heldValue{name: secret.Name, secret: true, updatedAt: asTime(secret.UpdatedAt)})
		}
		offset += len(listing.Secrets)
		if len(listing.Secrets) == 0 || offset >= listing.TotalResults {
			break
		}
	}
	return held, nil
}

// pageAt asks for the largest page the store answers, from offset.
func pageAt(offset int) StoreListQuery {
	return StoreListQuery{Limit: strconv.Itoa(maxNamesPerLookup), Offset: strconv.Itoa(offset)}
}

// keptValues is what one sweep keeps: every name something refers to, and the ID segments of the
// resources the export of the current configuration left out.
type keptValues struct {
	names map[string]struct{}
	// leftOutIDs are the ID segments, as varname spells them, of the resources the export left out,
	// and leftOut how many resources it left out.
	leftOutIDs map[string]struct{}
	leftOut    int
}

// ofLeftOutResource reports whether a generated name belongs to a resource the export left out: its ID
// segment is that resource's.
func (k keptValues) ofLeftOutResource(name string) bool {
	if len(k.leftOutIDs) == 0 {
		return false
	}
	segment, ok := varname.IDSegmentOf(name)
	if !ok {
		return false
	}
	_, leftOut := k.leftOutIDs[segment]
	return leftOut
}

// referenced returns every name the configuration as it stands, a version the deployment keeps, or a
// master the gateway keeps refers to, as a variable or a secret, with the resources the export of the
// configuration left out. A name is kept whichever it is referred to as. Any of them that cannot be
// read stops the sweep, since a value it refers to would otherwise look unused.
func (s *unusedValueSweeper) referenced(ctx context.Context, gatewayID string) (keptValues, error) {
	names := map[string]struct{}{}
	versions, err := s.versions.List(ctx)
	if err != nil {
		return keptValues{}, fmt.Errorf("failed to list the versions: %w", err)
	}
	seqs := make(map[int]string, len(versions)+2)
	for _, version := range versions {
		seqs[version.Seq] = version.Hash
	}
	// What the gateway holds and reverts to is kept by the versions' own retention; read here as well,
	// so neither is missed should a listing leave it out.
	applied, err := s.versions.GetApplied(ctx, gatewayID)
	if err != nil {
		return keptValues{}, fmt.Errorf("failed to read what the gateway holds: %w", err)
	}
	if applied != nil {
		for _, seq := range []int{applied.AppliedVersion, applied.PreviousVersion} {
			if _, listed := seqs[seq]; seq > 0 && !listed {
				seqs[seq] = ""
			}
		}
	}
	read := 0
	for seq, hash := range seqs {
		found, err := s.versionReferences(ctx, seq, hash, names)
		if err != nil {
			return keptValues{}, err
		}
		if found {
			read++
		}
	}
	if read == 0 {
		return keptValues{}, errNoRetainedVersion
	}

	masters, err := s.versions.ListMasters(ctx, gatewayID)
	if err != nil {
		return keptValues{}, fmt.Errorf("failed to list the gateway's masters: %w", err)
	}
	for _, master := range masters {
		cacheKey := "master/" + gatewayID + "/" + strconv.Itoa(master.Seq) + "/" + master.Hash
		set, ok := s.references.get(cacheKey)
		if !ok {
			full, err := s.versions.GetMasterAt(ctx, gatewayID, master.Seq)
			if err != nil {
				return keptValues{}, fmt.Errorf("failed to read the gateway's master: %w", err)
			}
			if full == nil {
				// Removed since it was listed: nothing can be reverted to it any more.
				continue
			}
			set = namesIn(full.Resources)
			s.references.put(cacheKey, set)
		}
		maps.Copy(names, set)
	}

	// Read last, so a resource written while the versions and masters were read is in it.
	current, err := s.currentReferences(ctx)
	if err != nil {
		return keptValues{}, err
	}
	maps.Copy(names, current.names)
	current.names = names
	return current, nil
}

// currentReferences exports the configuration as it stands, as a capture would, and returns every name
// it refers to, with the ID segments of the resources it left out, such as one deleted while the
// export ran, so their values are kept too. An export that fails or holds nothing is no answer, and
// neither is one that left out something it names no ID for: a value of what it missed would look
// unused.
func (s *unusedValueSweeper) currentReferences(ctx context.Context) (keptValues, error) {
	exported, svcErr := s.current.exportConfiguration(ctx)
	if svcErr != nil {
		return keptValues{}, fmt.Errorf("%w: the export failed with %s", errNoCurrentConfiguration, svcErr.Code)
	}
	if exported == nil || len(exported.Files) == 0 {
		return keptValues{}, fmt.Errorf("%w: the export holds nothing", errNoCurrentConfiguration)
	}
	kept := keptValues{names: namesIn(joinBundle(documentsOf(exported)))}
	if exported.Summary == nil || len(exported.Summary.Errors) == 0 {
		return kept, nil
	}
	kept.leftOut = len(exported.Summary.Errors)
	kept.leftOutIDs = make(map[string]struct{}, kept.leftOut)
	for _, failed := range exported.Summary.Errors {
		if failed.ResourceID == "" {
			return keptValues{}, fmt.Errorf("%w: the export left out a %s resource without naming it",
				errNoCurrentConfiguration, failed.ResourceType)
		}
		kept.leftOutIDs[varname.IDSegment(failed.ResourceID)] = struct{}{}
	}
	return kept, nil
}

// versionReferences adds the names one version refers to, and reports whether the version is still
// kept. hash is the version's hash as listed, empty when it was not.
func (s *unusedValueSweeper) versionReferences(ctx context.Context, seq int, hash string,
	names map[string]struct{}) (bool, error) {
	if hash != "" {
		if set, ok := s.references.get("version/" + hash); ok {
			maps.Copy(names, set)
			return true, nil
		}
	}
	version, err := s.versions.Get(ctx, seq)
	if err != nil {
		return false, fmt.Errorf("failed to read a version: %w", err)
	}
	if version == nil {
		// Removed since it was listed: it can no longer be applied.
		return false, nil
	}
	set := namesIn(version.Resources)
	// A version's hash is the hash of its content, so the hash it is read with names what it holds,
	// even when the current version was replaced in place since it was listed.
	s.references.put("version/"+version.Hash, set)
	maps.Copy(names, set)
	return true, nil
}

// namesIn is every name a configuration refers to, as a variable or a secret.
func namesIn(content string) map[string]struct{} {
	variables, secrets := referencesIn(content)
	set := make(map[string]struct{}, len(variables)+len(secrets))
	for _, name := range variables {
		set[name] = struct{}{}
	}
	for _, name := range secrets {
		set[name] = struct{}{}
	}
	return set
}

// referenceCache holds the names each version and master refers to, by a key that changes whenever
// what it holds does.
type referenceCache struct {
	mu   sync.Mutex
	sets map[string]map[string]struct{}
}

func (c *referenceCache) get(key string) (map[string]struct{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	set, ok := c.sets[key]
	return set, ok
}

func (c *referenceCache) put(key string, set map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.sets) >= maxCachedReferenceSets {
		c.sets = map[string]map[string]struct{}{}
	}
	c.sets[key] = set
}
