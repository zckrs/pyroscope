package vcs

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/go-kit/log"
	giturl "github.com/kubescape/go-git-url"

	vcsv1 "github.com/grafana/pyroscope/api/gen/proto/go/vcs/v1"
	vcsv1connect "github.com/grafana/pyroscope/api/gen/proto/go/vcs/v1/vcsv1connect"
	"github.com/grafana/pyroscope/pkg/querier/vcs/github"
	"github.com/grafana/pyroscope/pkg/querier/vcs/source"
)

var (
	_ vcsv1connect.VCSServiceHandler = (*Service2)(nil)

	supportedGitProviders = []string{
		"github",
	}
)

func New2(logger log.Logger) (*Service2, error) {
	ghClient, err := github.NewClient()
	if err != nil {
		return nil, err
	}

	finder, err := source.NewFinder(logger, ghClient)
	if err != nil {
		return nil, err
	}

	svc := &Service2{
		logger:       logger,
		githubClient: ghClient,
		finder:       finder,
	}
	return svc, nil
}

type Service2 struct {
	logger       log.Logger
	githubClient github.Client
	finder       source.Finder
}

func (s *Service2) GithubApp(ctx context.Context, req *connect.Request[vcsv1.GithubAppRequest]) (*connect.Response[vcsv1.GithubAppResponse], error) {
	clientID, err := s.githubClient.AppClientID(ctx)
	if err != nil {
		s.logger.Log("err", err, "msg", "failed to fetch client id")
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to fetch client id"))
	}

	res := &vcsv1.GithubAppResponse{
		ClientID: clientID,
	}
	return connect.NewResponse(res), nil
}

func (s *Service2) GithubLogin(context.Context, *connect.Request[vcsv1.GithubLoginRequest]) (*connect.Response[vcsv1.GithubLoginResponse], error) {
	panic("unimplemented")
}

func (s *Service2) GithubRefresh(context.Context, *connect.Request[vcsv1.GithubRefreshRequest]) (*connect.Response[vcsv1.GithubRefreshResponse], error) {
	panic("unimplemented")
}

func (s *Service2) GetCommit(ctx context.Context, req *connect.Request[vcsv1.GetCommitRequest]) (*connect.Response[vcsv1.GetCommitResponse], error) {
	token, err := tokenFromRequest(req)
	if err != nil {
		s.logger.Log("err", err, "msg", "failed to extract token from request")
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
	}

	url, err := getGitProviderURL(req.Msg.RepositoryURL)
	if err != nil {
		s.logger.Log("err", err, "msg", "failed to get git provider")
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid repository url: %s", req.Msg.RepositoryURL))
	}

	commit, err := s.githubClient.GetCommit(ctx, token.AccessToken, github.GetCommitParams{
		Owner: url.GetOwnerName(),
		Repo:  url.GetRepoName(),
		Ref:   req.Msg.Ref,
	})
	if err != nil {
		s.logger.Log("err", err, "msg", "failed to get commit")
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get commit"))
	}

	res := &vcsv1.GetCommitResponse{
		Message: commit.Message,
		Author: &vcsv1.CommitAuthor{
			Login:     commit.Author.Login,
			AvatarURL: commit.Author.AvatarURL,
		},
		Date: commit.Date,
		Sha:  commit.Sha,
		URL:  commit.URL,
	}
	return connect.NewResponse(res), nil
}

func (s *Service2) GetFile(ctx context.Context, req *connect.Request[vcsv1.GetFileRequest]) (*connect.Response[vcsv1.GetFileResponse], error) {
	token, err := tokenFromRequest(req)
	if err != nil {
		s.logger.Log("err", err, "msg", "failed to extract token from request")
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
	}

	url, err := getGitProviderURL(req.Msg.RepositoryURL)
	if err != nil {
		s.logger.Log("err", err, "msg", "failed to get git provider")
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid repository url: %s", req.Msg.RepositoryURL))
	}

	file, err := s.finder.Find(ctx, token.AccessToken, url, req.Msg.Ref, req.Msg.LocalPath)
	if err != nil {
		s.logger.Log("err", err, "msg", "filename", req.Msg.LocalPath, "failed to find file")
		if errors.Is(err, source.ErrFileNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to find file: %s", req.Msg.LocalPath))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find file: %s", req.Msg.LocalPath))
	}

	res := &vcsv1.GetFileResponse{
		Content: file.Content,
		URL:     file.URL,
	}
	return connect.NewResponse(res), nil
}

func getGitProviderURL(repoURL string) (giturl.IGitURL, error) {
	url, err := giturl.NewGitURL(repoURL)
	if err != nil {
		return nil, err
	}

	for _, provider := range supportedGitProviders {
		if url.GetProvider() == provider {
			return url, err
		}
	}
	return nil, fmt.Errorf("unsupported git provider, supported providers: %v", supportedGitProviders)
}
