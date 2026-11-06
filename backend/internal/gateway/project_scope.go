// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// A deploy can be scoped to one project. A project's own resources are those whose documents name it as
// their project, such as its applications and agents. What they need is read from the version: every
// resource they name by id, followed through what those name in turn. What names no project is shared
// by the deployment's projects, such as organization units, translations, branding, the flows no
// resource of the project uses and system users, and is sent with a project only when selected. Another
// project's own resources are that project's to deploy, and are never sent with this one.
const (
	sectionProject = "project"
	sectionNeeded  = "needed"
	sectionShared  = "shared"
	sectionOther   = "other"
)

// projectIDField reads the project a resource names as its own.
var projectIDField = topLevelScalar("projectId")

// resourceTypeProject is the resource type a project's own document is exported under.
const resourceTypeProject = "project"

// ProjectDirectory reads the projects of the deployment a deploy can be scoped to. Only a control plane
// keeps projects; without a directory no project exists.
type ProjectDirectory interface {
	// ProjectExists reports whether the deployment has the project.
	ProjectExists(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
	// ProjectNames maps each of the deployment's projects, by id, to the name it is shown by.
	ProjectNames(ctx context.Context) (map[string]string, *tidcommon.ServiceError)
}

// noProjects is the directory of a plane that keeps no projects.
type noProjects struct{}

func (noProjects) ProjectExists(context.Context, string) (bool, *tidcommon.ServiceError) {
	return false, nil
}

func (noProjects) ProjectNames(context.Context) (map[string]string, *tidcommon.ServiceError) {
	return map[string]string{}, nil
}

// projectScope is what a deploy scoped to a project sends by default and must send.
type projectScope struct {
	projectID string
	// defaults is the project's own changes, except those the gateway holds back, and the required ones.
	defaults []string
	// required is the needed changes the gateway lacks or refused: the project's resources cannot run
	// without them.
	required map[string]bool
	// others is the changes of other projects' own resources, which this deploy may not send.
	others map[string]bool
	owners map[string]string
	needs  map[string][]string
}

// ownersOf maps each resource that belongs to a project to that project, by key: a project's own
// document, and a resource whose document names the project. Later documents win, so a version read
// after what the gateway holds decides for a resource both have.
func ownersOf(sets ...[]bundleResource) map[string]string {
	owners := map[string]string{}
	for _, resources := range sets {
		for _, resource := range resources {
			if owner := ownerOf(resource); owner != "" {
				owners[resource.key()] = owner
			} else {
				delete(owners, resource.key())
			}
		}
	}
	return owners
}

// ownerOf reads the project one resource belongs to, or "" when it belongs to none.
func ownerOf(resource bundleResource) string {
	if resource.Type == resourceTypeProject {
		return resource.ID
	}
	return firstMatch(projectIDField, resource.Content)
}

// soleNeeders maps each resource that belongs to no project, but that the resources of exactly one
// project need, to that project. What a project's resources need is followed through what that needs in
// turn, never through another project's own resources. Such a resource goes with that project's sync and
// counts in its status, not as an update shared with every project; one needed by several projects, or
// by none, stays shared.
func soleNeeders(resources []bundleResource) map[string]string {
	owners := ownersOf(resources)
	needs := needsOf(resources)
	reachers := map[string]map[string]bool{}
	for key, owner := range owners {
		queue := append([]string{}, needs[key]...)
		visited := map[string]bool{}
		for len(queue) > 0 {
			next := queue[0]
			queue = queue[1:]
			if visited[next] || owners[next] != "" {
				continue
			}
			visited[next] = true
			if reachers[next] == nil {
				reachers[next] = map[string]bool{}
			}
			reachers[next][owner] = true
			queue = append(queue, needs[next]...)
		}
	}
	sole := map[string]string{}
	for key, projects := range reachers {
		if len(projects) != 1 {
			continue
		}
		for project := range projects {
			sole[key] = project
		}
	}
	return sole
}

// scopeToProject places each change of a diff in the project's sections and marks the needed changes the
// gateway lacks or refused as required. Who owns a resource is read from the version, or, for one the
// version removes, from what the gateway holds.
func scopeToProject(projectID string, owners map[string]string, diff *Diff) projectScope {
	scope := projectScope{projectID: projectID, required: map[string]bool{}, others: map[string]bool{},
		owners: owners, needs: map[string][]string{}}
	for _, change := range diff.Changes {
		scope.needs[change.Key] = change.Needs
	}
	var own []string
	for key, owner := range owners {
		if owner == projectID {
			own = append(own, key)
		}
	}
	needed := scope.reach(own)
	for i := range diff.Changes {
		change := &diff.Changes[i]
		owner := owners[change.Key]
		switch {
		case owner == projectID:
			change.Section = sectionProject
			if change.Change != ChangeUnchanged && !change.HeldBack {
				scope.defaults = append(scope.defaults, change.Key)
			}
		case owner != "":
			change.Section = sectionOther
			scope.others[change.Key] = true
		case needed[change.Key]:
			change.Section = sectionNeeded
			// The project's resources cannot run without what the gateway lacks or refused; one it holds
			// in another form serves them, and replacing it is a choice of its own. One the gateway holds
			// back stays required but is sent only when selected, which stops holding it back.
			if change.Change == ChangeAdded || change.Failed {
				change.Required = true
				scope.required[change.Key] = true
				if !change.HeldBack {
					scope.defaults = append(scope.defaults, change.Key)
				}
			}
		default:
			change.Section = sectionShared
		}
	}
	return scope
}

// reach follows what the given keys need, through what that needs in turn, leaving out the project's
// own resources.
func (s projectScope) reach(from []string) map[string]bool {
	reached := map[string]bool{}
	queue := append([]string{}, from...)
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		for _, need := range s.needs[key] {
			// What any project owns is its own to deploy, so what it names is not followed either.
			if s.owners[need] != "" || reached[need] {
				continue
			}
			reached[need] = true
			queue = append(queue, need)
		}
	}
	return reached
}

// check refuses a selection that names another project's own resource, or that leaves out a required
// change a selected resource of the project needs.
func (s projectScope) check(selection []string) *tidcommon.ServiceError {
	selected := toSet(selection)
	var others, own []string
	for _, key := range selection {
		if s.others[key] {
			others = append(others, key)
		}
		if s.owners[key] == s.projectID {
			own = append(own, key)
		}
	}
	if len(others) > 0 {
		sort.Strings(others)
		return tidcommon.CustomServiceError(ErrorSelectionOfOtherProject, tidcommon.I18nMessage{
			Key: ErrorSelectionOfOtherProject.ErrorDescription.Key,
			DefaultValue: byCount(len(others),
				strings.Join(others, ", ")+" belongs to another project, which deploys it itself. "+
					"Leave it out of the selection.",
				strings.Join(others, ", ")+" belong to other projects, which deploy them themselves. "+
					"Leave them out of the selection.")})
	}
	var missing []string
	for key := range s.reach(own) {
		if s.required[key] && !selected[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return tidcommon.CustomServiceError(ErrorProjectNeedsLeftOut, tidcommon.I18nMessage{
		Key: ErrorProjectNeedsLeftOut.ErrorDescription.Key,
		DefaultValue: "The project's resources need " + strings.Join(missing, ", ") + byCount(len(missing),
			", which the gateway does not hold. Select it as well.",
			", which the gateway does not hold. Select them as well.")})
}

// byCount is the message for one, or the one for several.
func byCount(count int, one, several string) string {
	if count == 1 {
		return one
	}
	return several
}

// markUsedBy names, on each needed or shared change that updates or removes a resource the gateway holds,
// the other projects whose resources on the gateway name that resource by id, so a caller sees whom
// deploying it would also change. What the gateway holds is its master, not the version it is recorded
// as running, which can hold resources a selection left out.
func (s projectScope) markUsedBy(diff *Diff, held []bundleResource, projects map[string]string) {
	for i := range diff.Changes {
		change := &diff.Changes[i]
		if (change.Section != sectionNeeded && change.Section != sectionShared) || change.ID == "" ||
			(change.Change != ChangeUpdated && change.Change != ChangeDeleted) {
			continue
		}
		users := map[string]bool{}
		for _, resource := range held {
			owner := s.owners[resource.key()]
			if owner == "" || owner == s.projectID || resource.key() == change.Key ||
				!strings.Contains(resource.Content, change.ID) {
				continue
			}
			users[owner] = true
		}
		if len(users) == 0 {
			continue
		}
		for owner := range users {
			name := projects[owner]
			if name == "" {
				name = owner
			}
			change.UsedBy = append(change.UsedBy, name)
		}
		sort.Strings(change.UsedBy)
	}
}

// scope places a diff's changes in a project's sections, after checking the project exists.
func (s *versionService) scope(ctx context.Context, projectID string, diff *Diff, target *Version,
	master *Master) (projectScope, *tidcommon.ServiceError) {
	found, svcErr := s.projects.ProjectExists(ctx, projectID)
	if svcErr != nil {
		return projectScope{}, svcErr
	}
	if !found {
		return projectScope{}, &ErrorProjectNotFound
	}
	projects, svcErr := s.projects.ProjectNames(ctx)
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to read the projects", log.String("errorCode", svcErr.Code))
		projects = map[string]string{}
	}
	held := parseBundle(master.Resources)
	owners := ownersOf(held, parseBundle(target.Resources))
	scope := scopeToProject(projectID, owners, diff)
	scope.markUsedBy(diff, held, projects)
	return scope, nil
}

// scopedSelection settles what a deploy scoped to a project sends: the caller's selection, or else the
// project's defaults, narrowed to what a promotion may send when it chooses from the diff. It is
// refused when it names another project's resource or leaves out what the project's resources need.
func scopedSelection(scope projectScope, requested *[]string, promotable *[]string) ([]string,
	*tidcommon.ServiceError) {
	wanted := scope.defaults
	if requested != nil {
		wanted = *requested
	}
	if promotable != nil {
		allowed := toSet(*promotable)
		narrowed := []string{}
		for _, key := range wanted {
			if allowed[key] {
				narrowed = append(narrowed, key)
			}
		}
		wanted = narrowed
	}
	if wanted == nil {
		wanted = []string{}
	}
	if svcErr := scope.check(wanted); svcErr != nil {
		return nil, svcErr
	}
	return wanted, nil
}
