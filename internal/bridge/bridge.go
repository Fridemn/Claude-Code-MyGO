package bridge

type Client struct {
	enabled bool
	mode    string
}

func CreateClient() *Client {
	return &Client{
		enabled: false,
		mode:    "local",
	}
}

func (c *Client) Enabled() bool {
	return c.enabled
}

func (c *Client) Mode() string {
	return c.mode
}
