package source

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/go-kit/log"
	giturl "github.com/kubescape/go-git-url"

	"github.com/grafana/pyroscope/pkg/querier/vcs/github"
)

const (
	defaultRef = "HEAD"
	extGo      = ".go"
)

var (
	ErrFileNotFound = errors.New("not found")
)

type File struct {
	Content string
	URL     string
}

type Finder interface {
	Find(ctx context.Context, userToken string, url giturl.IGitURL, ref string, path string) (File, error)
}

func NewFinder(logger log.Logger, client github.Client) (Finder, error) {
	f := &finder{}

	return f, nil
}

type finder struct {
	logger       log.Logger
	githubClient github.Client
}

func (f *finder) Find(ctx context.Context, userToken string, url giturl.IGitURL, ref string, path string) (File, error) {
	if ref == "" {
		ref = defaultRef
	}

	fetcher := &fileFetcher{
		logger:       f.logger,
		githubClient: f.githubClient,
		accessToken:  userToken,
		url:          url,
		ref:          ref,
		path:         path,
	}

	// todo: add more languages support
	switch filepath.Ext(path) {
	case extGo:
		return fetcher.FetchGoFile(ctx)
	default:
		// By default we return the file content at the given path without any
		// processing.
		return fetcher.FetchRepoFile(ctx)
	}
}

type fileFetcher struct {
	logger       log.Logger
	githubClient github.Client
	accessToken  string
	url          giturl.IGitURL
	ref          string
	path         string
}

func (ff *fileFetcher) FetchGoFile(ctx context.Context) (File, error) {
	// TODO: copy old file finder implementation here
	panic("unimplemented")
}

func (ff *fileFetcher) FetchRepoFile(ctx context.Context) (File, error) {
	panic("unimplemented")
}
