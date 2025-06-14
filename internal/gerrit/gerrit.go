package gerrit

import (
	"context"

	gerrit"github.com/andygrunwald/go-gerrit"
)

type Client struct {
	client *gerrit.Client
}

func NewClient(baseURL string) *Client {
	ctx := context.Background()
	client, err := gerrit.NewClient(ctx, baseURL, nil)
	if err != nil {
		panic(err)
	}
	return &Client{
		client: client,
	}
}
