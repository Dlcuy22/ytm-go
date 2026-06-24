package ytm

import "context"

func (c *Client) GetSearchContinuation(ctx context.Context, token string) ([]MediaItem, string, error) {
	items, cont, err := c.GetPlaylistContinuation(ctx, false, token, 0)
	if err != nil {
		return nil, "", err
	}
	nextToken := ""
	if cont != nil {
		nextToken = cont.Token
	}
	return items, nextToken, nil
}
