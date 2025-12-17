package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synacktiv/octoscan/common"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type GitHub struct {
	client            *github.Client
	ctx               context.Context
	path              string
	org               string
	repo              string
	outputDir         string
	defaultBranchOnly bool
	maxBranches       int
	includeArchives   bool
	includeForks      bool
}

type GitHubOptions struct {
	Proxy             bool
	Token             string
	Path              string
	Org               string
	Repo              string
	OutputDir         string
	DefaultBranchOnly bool
	MaxBranches       int
	IncludeArchives   bool
	IncludeForks      bool
}

func NewGitHub(opts GitHubOptions) *GitHub {
	var tc *http.Client

	ctx := context.Background()

	if opts.Token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Token})
		tc = oauth2.NewClient(ctx, ts)
	}

	return &GitHub{
		client:            github.NewClient(tc),
		ctx:               ctx,
		path:              opts.Path,
		org:               opts.Org,
		repo:              opts.Repo,
		outputDir:         opts.OutputDir,
		defaultBranchOnly: opts.DefaultBranchOnly,
		maxBranches:       opts.MaxBranches,
		includeForks:      opts.IncludeForks,
	}
}

func (gh *GitHub) Download() error {
	// get all repos with pagination
	var allRepos []*github.Repository

	var err error

	allRepos, err = gh.getRepos()

	if err != nil {
		return err
	}

	for _, repo := range allRepos {
		if !gh.includeForks && repo.GetFork() {
			common.Log.Debug(fmt.Sprintf("Not including %s because it's a fork", repo.GetName()))

			continue
		}

		if !gh.includeArchives && *repo.Archived {
			common.Log.Debug(fmt.Sprintf("Not including %s because it has been archived", repo.GetName()))

			continue
		}

		// check rate limit
		err := gh.checkRateLimit()
		if err != nil {
			return err
		}

		err = gh.DownloadRepo(repo)
		if err != nil {
			common.Log.Error(fmt.Sprintf("Error while downloading files of repo: %s", repo.GetName()))
		}
	}

	return nil
}

func (gh *GitHub) getRepos() ([]*github.Repository, error) {
	var allRepos []*github.Repository

	var err error

	// we only want one repo
	if gh.repo != "" {
		repo, err := gh.getSingleRepo(gh.repo)
		if err != nil {
			return nil, err
		}

		if repo != nil {
			allRepos = append(allRepos, repo)
		}
	} else {
		// we want all the repositories
		allRepos, err = gh.getOrgOrUserRepos()

		if err != nil {
			return nil, err
		}
	}

	return allRepos, nil
}

func (gh *GitHub) getSingleRepo(repo string) (*github.Repository, error) {
	repository, _, err := gh.client.Repositories.Get(gh.ctx, gh.org, repo)
	if err != nil {
		common.Log.Error(fmt.Sprintf("Fail to find repository %s: %v", repo, err))

		return nil, err
	}

	return repository, nil
}

func (gh *GitHub) getOrgOrUserRepos() ([]*github.Repository, error) {
	var allRepos []*github.Repository

	common.Log.Info(fmt.Sprintf("Downloading files of org: %s", gh.org))

	user, _, err := gh.client.Users.Get(gh.ctx, gh.org)
	if err != nil {
		common.Log.Error(fmt.Sprintf("Fail to determine if %s is a user or an org: %v", gh.org, err))

		return nil, err
	}

	if user.GetType() == "Organization" {
		allRepos, err = gh.getOrgRepos()
	} else {
		allRepos, err = gh.getUserRepos()
	}

	return allRepos, err
}

func (gh *GitHub) getOrgRepos() ([]*github.Repository, error) {
	opt := &github.RepositoryListByOrgOptions{}

	var allRepos []*github.Repository

	for {
		repos, resp, err := gh.client.Repositories.ListByOrg(gh.ctx, gh.org, opt)

		if err != nil {
			common.Log.Error(fmt.Sprintf("Fail to list repositories of org %s: %v", gh.org, err))

			return nil, err
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

func (gh *GitHub) getUserRepos() ([]*github.Repository, error) {
	opt := &github.RepositoryListOptions{}

	var allRepos []*github.Repository

	for {
		repos, resp, err := gh.client.Repositories.List(gh.ctx, gh.org, opt)

		if err != nil {
			common.Log.Error(fmt.Sprintf("Fail to list repositories of org %s: %v", gh.org, err))

			return nil, err
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

func (gh *GitHub) DownloadRepo(repository *github.Repository) error {
	// check rate limit
	err := gh.checkRateLimit()
	if err != nil {
		return err
	}

	allBranches := []struct {
		Name string
		SHA  string
	}{}

	opt := &github.ListOptions{}

	common.Log.Info(fmt.Sprintf("Downloading files of repo: %s", repository.GetName()))

	if gh.defaultBranchOnly {
		ref, _, err := gh.client.Git.GetRef(gh.ctx, gh.org, repository.GetName(), "refs/heads/"+*repository.DefaultBranch)
		if err != nil {
			common.Log.Error(fmt.Sprintf("Fail to get default branche of repository %s: %v", repository.GetName(), err))

			return err
		}
		allBranches = append(allBranches, struct {
			Name string
			SHA  string
		}{Name: *repository.DefaultBranch, SHA: *ref.Object.SHA})
	} else {
		for {
			branches, resp, err := gh.client.Repositories.ListBranches(gh.ctx, gh.org, repository.GetName(), opt)

			if err != nil {
				common.Log.Error(fmt.Sprintf("Fail to list branches of repository %s: %v", repository.GetName(), err))

				return err
			}

			for _, branch := range branches {
				if branch.Name != nil && branch.Commit != nil && branch.Commit.SHA != nil {
					allBranches = append(allBranches, struct {
						Name string
						SHA  string
					}{Name: *branch.Name, SHA: *branch.Commit.SHA})
				}
			}

			// truncate array for repos with too much branches
			if gh.maxBranches != 0 && len(allBranches) >= gh.maxBranches {
				allBranches = allBranches[:gh.maxBranches]

				break
			}

			if resp.NextPage == 0 {
				break
			}

			opt.Page = resp.NextPage
		}
	}
	for _, branch := range allBranches {
		// check rate limit
		err := gh.checkRateLimit()
		if err != nil {
			return err
		}

		err = gh.DownloadContentFromBranch(repository.GetName(), branch.Name, branch.SHA)
		if err != nil {
			common.Log.Error(err)
		}
	}

	return nil
}

func (gh *GitHub) DownloadContentFromBranch(repo, branch, commit string) error {
	// create the dir for output
	fp := filepath.Join(gh.outputDir, gh.org, repo, branch)
	_ = os.MkdirAll(fp, 0755)

	// used for the scanner
	_, _ = os.Create(filepath.Join(fp, ".git"))

	return gh.downloadDirectory(repo, branch, commit, gh.path)
}

func (gh *GitHub) downloadRawFile(repo, branch, commit, path string) error {
	url := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/%s/%s",
		gh.org,
		repo,
		commit,
		path,
	)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("raw download failed (%s): %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// file may not exist in this branch – this is normal
		common.Log.Verbose(fmt.Sprintf("Skipping %s (%s)", path, resp.Status))
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	dst := filepath.Join(gh.outputDir, gh.org, repo, branch, path)
	_ = os.MkdirAll(filepath.Dir(dst), 0755)

	return os.WriteFile(dst, data, 0600)
}

func (gh *GitHub) downloadDirectory(repo, branch, commit, path string) error {
	tree, _, err := gh.client.Git.GetTree(gh.ctx, gh.org, repo, commit, true)
	if err != nil {
		return fmt.Errorf("failed to get tree for branch %s (commit %s): %w", branch, commit, err)
	}

	if tree.GetTruncated() {
		common.Log.Info(fmt.Sprintf("Tree truncated for %s/%s/%s, falling back to API", gh.org, repo, branch))
		return gh.downloadDirectoryFallback(repo, branch, commit, path)
	}

	for _, entry := range tree.Entries {
		if *entry.Type != "blob" {
			continue
		}

		if !strings.HasPrefix(*entry.Path, path+"/") && *entry.Path != path {
			continue
		}

		if err := gh.downloadRawFile(repo, branch, commit, *entry.Path); err != nil {
			common.Log.Error(err)
		}
	}

	return nil
}

// Fallback: Old implementation
func (gh *GitHub) downloadDirectoryFallback(repo, branch, commit, path string) error {
	_, directoryContent, _, err := gh.client.Repositories.GetContents(
		gh.ctx,
		gh.org,
		repo,
		path,
		&github.RepositoryContentGetOptions{Ref: commit},
	)
	if err != nil {
		return err
	}

	for _, element := range directoryContent {
		switch element.GetType() {
		case "dir":
			err = gh.downloadDirectory(repo, branch, commit, element.GetPath())
			if err != nil {
				return err
			}
		case "file":
			err = gh.downloadRawFile(repo, branch, commit, element.GetPath())
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown type %s", element.GetType())
		}
	}

	return nil
}

func saveFileToDisk(content string, path string) error {
	// create the dir for output
	// TODO
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	err := os.WriteFile(path, []byte(content), 0600)

	if err != nil {
		return fmt.Errorf("error writing file (%s): %w", path, err)
	}

	return nil
}

func (gh *GitHub) checkRateLimit() error {
	// check rate limit
	rateLimit, _, err := gh.client.RateLimits(gh.ctx)

	if err != nil {
		common.Log.Error("Could not get rate limit.")

		return err
	}

	if rateLimit.Core.Remaining < 10 {
		common.Log.Info(fmt.Sprintf("Remaining %d requests before reaching GitHub max rate limit.", rateLimit.Core.Remaining))
		common.Log.Info(fmt.Sprintf("Sleeping %v minutes to refresh rate limit.", time.Until(rateLimit.Core.Reset.Time).Minutes()))
		time.Sleep(time.Until(rateLimit.Core.Reset.Time.Add(5 * time.Minute)))
	}

	return nil
}
