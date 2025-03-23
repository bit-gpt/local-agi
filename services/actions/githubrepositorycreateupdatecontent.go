package actions

import (
	"context"
	"fmt"

	"github.com/google/go-github/v69/github"
	"github.com/mudler/LocalAgent/core/types"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubRepositoryCreateOrUpdateContent struct {
	token, repository, owner, customActionName, defaultBranch, commitAuthor, commitMail string
	context                                                                             context.Context
	client                                                                              *github.Client
}

func NewGithubRepositoryCreateOrUpdateContent(ctx context.Context, config map[string]string) *GithubRepositoryCreateOrUpdateContent {
	client := github.NewClient(nil).WithAuthToken(config["token"])

	return &GithubRepositoryCreateOrUpdateContent{
		client:           client,
		token:            config["token"],
		repository:       config["repository"],
		owner:            config["owner"],
		customActionName: config["customActionName"],
		commitAuthor:     config["commitAuthor"],
		commitMail:       config["commitMail"],
		defaultBranch:    config["defaultBranch"],
		context:          ctx,
	}
}

func (g *GithubRepositoryCreateOrUpdateContent) Run(ctx context.Context, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		Path          string `json:"path"`
		Repository    string `json:"repository"`
		Owner         string `json:"owner"`
		Content       string `json:"content"`
		Branch        string `json:"branch"`
		CommitMessage string `json:"commit_message"`
	}{}
	err := params.Unmarshal(&result)
	if err != nil {
		fmt.Printf("error: %v", err)

		return types.ActionResult{}, err
	}

	if result.Branch == "" {
		result.Branch = "main"
	}

	if result.CommitMessage == "" {
		result.CommitMessage = "LocalAgent commit"
	}

	if g.repository != "" && g.owner != "" {
		result.Repository = g.repository
		result.Owner = g.owner
	}

	if g.defaultBranch != "" {
		result.Branch = g.defaultBranch
	}

	var sha *string
	c, _, _, _ := g.client.Repositories.GetContents(g.context, result.Owner, result.Repository, result.Path, nil)
	if c != nil {
		sha = c.SHA
	}

	fileContent, _, err := g.client.Repositories.CreateFile(g.context, result.Owner, result.Repository, result.Path, &github.RepositoryContentFileOptions{
		Message: &result.CommitMessage,
		SHA:     sha,
		Committer: &github.CommitAuthor{
			Name:  &g.commitAuthor,
			Email: &g.commitMail,
		},
		Branch:  &result.Branch,
		Content: []byte(result.Content),
	})
	if err != nil {
		resultString := fmt.Sprintf("Error creating content : %v", err)
		return types.ActionResult{Result: resultString}, err
	}

	return types.ActionResult{Result: fmt.Sprintf("File created/updated: %s\n", fileContent.GetURL())}, err
}

func (g *GithubRepositoryCreateOrUpdateContent) Definition() types.ActionDefinition {
	actionName := "github_repository_create_or_update_content"
	actionDescription := "Create or update a file in a GitHub repository"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	properties := map[string]jsonschema.Definition{
		"path": {
			Type:        jsonschema.String,
			Description: "The path to the file or directory",
		},
		"content": {
			Type:        jsonschema.String,
			Description: "The content to create/update",
		},
		"commit_message": {
			Type:        jsonschema.String,
			Description: "The commit message",
		},
	}

	if g.defaultBranch == "" {
		properties["branch"] = jsonschema.Definition{
			Type:        jsonschema.String,
			Description: "The branch to create/update the file",
		}
	}

	if g.repository != "" && g.owner != "" {
		return types.ActionDefinition{
			Name:        types.ActionDefinitionName(actionName),
			Description: actionDescription,
			Properties:  properties,
			Required:    []string{"path", "content"},
		}
	}

	properties["owner"] = jsonschema.Definition{
		Type:        jsonschema.String,
		Description: "The owner of the repository",
	}

	properties["repository"] = jsonschema.Definition{
		Type:        jsonschema.String,
		Description: "The repository to search in",
	}

	return types.ActionDefinition{
		Name:        types.ActionDefinitionName(actionName),
		Description: actionDescription,
		Properties:  properties,
		Required:    []string{"path", "repository", "owner", "content"},
	}
}

func (a *GithubRepositoryCreateOrUpdateContent) Plannable() bool {
	return true
}
