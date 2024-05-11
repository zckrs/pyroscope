package github

import (
	"context"
)

type Client interface {
	// AppClientID is the GitHub app client id.
	AppClientID(ctx context.Context) (string, error)

	// Authorize exchanges an authorization code for a user token.
	Authorize(ctx context.Context, code string) (AuthToken, error)

	// Refresh refreshes a user token for a new user token.
	Refresh(ctx context.Context, token AuthToken) (AuthToken, error)

	// GetCommit fetches a commit.
	GetCommit(ctx context.Context, userToken string, params GetCommitParams) (Commit, error)

	// GetFile fetches a file.
	GetFile(ctx context.Context, userToken string, params GetFileParams) (File, error)
}

// NewClient builds a new Client.
func NewClient() (Client, error) {
	client := &githubClient{}

	return client, nil
}

type githubClient struct{}

func (g *githubClient) AppClientID(ctx context.Context) (string, error) {
	panic("unimplemented")
}

func (g *githubClient) Authorize(ctx context.Context, code string) (AuthToken, error) {
	panic("unimplemented")
}

func (g *githubClient) Refresh(ctx context.Context, token AuthToken) (AuthToken, error) {
	panic("unimplemented")
}

func (g *githubClient) GetCommit(ctx context.Context, accessToken string, params GetCommitParams) (Commit, error) {
	panic("unimplemented")
}

func (g *githubClient) GetFile(ctx context.Context, accessToken string, params GetFileParams) (File, error) {
	panic("unimplemented")
}
