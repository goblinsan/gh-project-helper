package github

import (
	"context"
	"net/http"

	"github.com/google/go-github/v66/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// Client wraps both the REST API client (go-github) and GraphQL client (githubv4)
type Client struct {
	REST    *github.Client
	GraphQL *githubv4.Client
}

// NewClient creates a new GitHub client with both REST and GraphQL capabilities
func NewClient(token string) *Client {
	var httpClient *http.Client
	
	if token != "" {
		// Create an OAuth2 token source
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		httpClient = oauth2.NewClient(context.Background(), ts)
	} else {
		httpClient = http.DefaultClient
	}

	return &Client{
		REST:    github.NewClient(httpClient),
		GraphQL: githubv4.NewClient(httpClient),
	}
}

// GetAuthenticatedUser returns information about the authenticated user
func (c *Client) GetAuthenticatedUser(ctx context.Context) (*github.User, error) {
	user, _, err := c.REST.Users.Get(ctx, "")
	return user, err
}
