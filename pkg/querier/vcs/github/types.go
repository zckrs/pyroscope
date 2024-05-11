package github

type AuthToken struct{}

type Commit struct {
	Message string
	Author  CommitAuthor
	Date    string
	Sha     string
	URL     string
}

type CommitAuthor struct {
	Login     string
	AvatarURL string
}

type File struct{}

type GetCommitParams struct {
	Owner string
	Repo  string
	Ref   string
}

type GetFileParams struct{}
