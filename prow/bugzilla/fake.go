/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bugzilla

import (
	"errors"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/util/sets"
)

// Fake is a fake Bugzilla client with injectable fields
type Fake struct {
	EndpointString  string
	Bugs            map[int]Bug
	BugComments     map[int][]Comment
	BugErrors       sets.Int
	BugCreateErrors sets.String
	ExternalBugs    map[int][]ExternalBug
}

// Endpoint returns the endpoint for this fake
func (c *Fake) Endpoint() string {
	return c.EndpointString
}

// GetBug retrieves the bug, if registered, or an error, if set,
// or responds with an error that matches IsNotFound
func (c *Fake) GetBug(id int) (*Bug, error) {
	if c.BugErrors.Has(id) {
		return nil, errors.New("injected error getting bug")
	}
	if bug, exists := c.Bugs[id]; exists {
		return &bug, nil
	}
	return nil, &requestError{statusCode: http.StatusNotFound, message: "bug not registered in the fake"}
}

// GetBug retrieves the external bugs for the Bugzilla bug,
// if registered, or an error, if set, or responds with an
// error that matches IsNotFound
func (c *Fake) GetExternalBugPRsOnBug(id int) ([]ExternalBug, error) {
	if c.BugErrors.Has(id) {
		return nil, errors.New("injected error adding external bug to bug")
	}
	if _, exists := c.Bugs[id]; exists {
		return c.ExternalBugs[id], nil
	}
	return nil, &requestError{statusCode: http.StatusNotFound, message: "bug not registered in the fake"}
}

// UpdateBug updates the bug, if registered, or an error, if set,
// or responds with an error that matches IsNotFound
func (c *Fake) UpdateBug(id int, update BugUpdate) error {
	if c.BugErrors.Has(id) {
		return errors.New("injected error updating bug")
	}
	if bug, exists := c.Bugs[id]; exists {
		bug.Status = update.Status
		bug.Resolution = update.Resolution
		c.Bugs[id] = bug
		return nil
	}
	return &requestError{statusCode: http.StatusNotFound, message: "bug not registered in the fake"}
}

// AddPullRequestAsExternalBug adds an external bug to the Bugzilla bug,
// if registered, or an error, if set, or responds with an error that
// matches IsNotFound
func (c *Fake) AddPullRequestAsExternalBug(id int, org, repo string, num int) (bool, error) {
	if c.BugErrors.Has(id) {
		return false, errors.New("injected error adding external bug to bug")
	}
	if _, exists := c.Bugs[id]; exists {
		pullIdentifier := IdentifierForPull(org, repo, num)
		for _, bug := range c.ExternalBugs[id] {
			if bug.BugzillaBugID == id && bug.ExternalBugID == pullIdentifier {
				return false, nil
			}
		}
		c.ExternalBugs[id] = append(c.ExternalBugs[id], ExternalBug{
			BugzillaBugID: id,
			ExternalBugID: pullIdentifier,
		})
		return true, nil
	}
	return false, &requestError{statusCode: http.StatusNotFound, message: "bug not registered in the fake"}
}

// CreateBug creates a new bug and associated description comment given a BugCreate or and error
// if description is in BugCreateErrors set
func (c *Fake) CreateBug(bug *BugCreate) (int, error) {
	if c.BugCreateErrors.Has(bug.Description) {
		return 0, errors.New("injected error creating new bug")
	}
	// add new bug one ID newer than highest existing BugID
	newID := 0
	for k := range c.Bugs {
		if k >= newID {
			newID = k + 1
		}
	}
	newBug := Bug{
		Alias:           bug.Alias,
		AssignedTo:      bug.AssignedTo,
		CC:              bug.CC,
		Component:       bug.Component,
		Flags:           bug.Flags,
		Groups:          bug.Groups,
		Keywords:        bug.Keywords,
		OperatingSystem: bug.OperatingSystem,
		Platform:        bug.Platform,
		Priority:        bug.Priority,
		Product:         bug.Product,
		QAContact:       bug.QAContact,
		Resolution:      bug.Resolution,
		Severity:        bug.Severity,
		Status:          bug.Status,
		Summary:         bug.Summary,
		TargetMilestone: bug.TargetMilestone,
		Version:         bug.Version,
	}
	c.Bugs[newID] = newBug
	// add new comment one ID newer than highest existing CommentID
	newCommentID := 0
	for _, comments := range c.BugComments {
		for _, comment := range comments {
			if comment.ID >= newCommentID {
				newCommentID = comment.ID + 1
			}
		}
	}
	newComments := []Comment{{
		ID:         newCommentID,
		BugID:      newID,
		Count:      0,
		Text:       bug.Description,
		IsMarkdown: bug.IsMarkdown,
		IsPrivate:  bug.CommentIsPrivate,
		Tags:       bug.CommentTags,
	}}
	c.BugComments[newID] = newComments
	return newID, nil
}

// GetBug retrieves the bug comments, if registered, or an error, if set,
// or responds with an error that matches IsNotFound
func (c *Fake) GetComments(id int) ([]Comment, error) {
	if c.BugErrors.Has(id) {
		return nil, errors.New("injected error getting bug comments")
	}
	if comments, exists := c.BugComments[id]; exists {
		return comments, nil
	}
	return nil, &requestError{statusCode: http.StatusNotFound, message: "bug comments not registered in the fake"}
}

func (c *Fake) CloneBug(bug *Bug) (int, error) {
	comments, err := c.GetComments(bug.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get parent bug's comments: %v", err)
	}
	return c.CreateBug(cloneBugStruct(bug, comments))
}

// the Fake is a Client
var _ Client = &Fake{}