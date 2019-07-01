// Copyright 2019 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:generate ../../../scripts/gen_topic.sh

package issues

import (
	"context"
	"html/template"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"istio.io/bots/policybot/dashboard"
	"istio.io/bots/policybot/pkg/storage"
	"istio.io/bots/policybot/pkg/storage/cache"
	"istio.io/bots/policybot/pkg/util"
)

type topic struct {
	store   storage.Store
	cache   *cache.Cache
	page    *template.Template
	context dashboard.RenderContext
	options *dashboard.Options
}

type IssueSummary struct {
	Repo        string   `json:"repo"`
	Number      int64    `json:"number"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	AuthorID    string   `json:"author_id"`
	AssigneeIDs []string `json:"assignee_ids"`
}

func NewTopic(store storage.Store, cache *cache.Cache) dashboard.Topic {
	return &topic{
		store: store,
		cache: cache,
		page:  template.Must(template.New("page").Parse(string(MustAsset("page.html")))),
	}
}

func (t *topic) Title() string {
	return "Issues"
}

func (t *topic) Description() string {
	return "Information on new and old issues."
}

func (t *topic) Name() string {
	return "issues"
}

func (t *topic) Configure(htmlRouter *mux.Router, apiRouter *mux.Router, context dashboard.RenderContext, opt *dashboard.Options) {
	t.context = context
	t.options = opt

	htmlRouter.StrictSlash(true).
		Path("/").
		Methods("GET").
		HandlerFunc(t.handleListIssuesHTML)

	apiRouter.StrictSlash(true).
		Path("/").
		Methods("GET").
		HandlerFunc(t.handleListIssuesJSON)
}

func (t *topic) handleListIssuesHTML(w http.ResponseWriter, r *http.Request) {
	orgLogin := r.URL.Query().Get("org")
	if orgLogin == "" {
		orgLogin = t.options.DefaultOrg
	}

	issues, err := t.getIssues(r.Context(), orgLogin)
	if err != nil {
		t.context.RenderHTMLError(w, err)
	}

	sb := &strings.Builder{}
	if err := t.page.Execute(sb, issues); err != nil {
		t.context.RenderHTMLError(w, err)
		return
	}

	t.context.RenderHTML(w, sb.String())
}

func (t *topic) handleListIssuesJSON(w http.ResponseWriter, r *http.Request) {
	orgLogin := r.URL.Query().Get("org")
	if orgLogin == "" {
		orgLogin = "istio" // TODO: remove istio dependency
	}

	issues, err := t.getIssues(r.Context(), orgLogin)
	if err != nil {
		t.context.RenderHTMLError(w, err)
		return
	}

	t.context.RenderJSON(w, http.StatusOK, issues)
}

func (t *topic) getIssues(context context.Context, orgLogin string) ([]IssueSummary, error) {
	org, err := t.cache.ReadOrgByLogin(context, orgLogin)
	if err != nil {
		return nil, util.HTTPErrorf(http.StatusInternalServerError, "unable to get information on organization %s: %v", orgLogin, err)
	} else if org == nil {
		return nil, util.HTTPErrorf(http.StatusNotFound, "no information available on organization %s", orgLogin)
	}

	repoName := "istio"
	repo, err := t.cache.ReadRepoByName(context, org.OrgID, repoName)
	if err != nil {
		return nil, util.HTTPErrorf(http.StatusInternalServerError, "unable to get information on repository %s: %v", repoName, err)
	} else if repo == nil {
		return nil, util.HTTPErrorf(http.StatusNotFound, "no information available on repository %s", repoName)
	}

	var issues []IssueSummary
	if err = t.store.QueryIssuesByRepo(context, org.OrgID, repo.RepoID, func(i *storage.Issue) error {
		issues = append(issues, IssueSummary{
			Repo:        repoName,
			Number:      i.Number,
			Title:       i.Title,
			State:       i.State,
			AuthorID:    i.AuthorID,
			AssigneeIDs: i.AssigneeIDs,
		})
		return nil
	}); err != nil {
		return nil, err
	}

	return issues, nil
}
