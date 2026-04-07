package mcp

type ChannelPermissions struct {
	Allowed map[string]bool `json:"allowed,omitempty"`
	Claimed map[string]bool `json:"-"`
}

type ChannelPermissionCallbacks struct {
	channel string
	perms   *ChannelPermissions
}

func DefaultChannelPermissions() ChannelPermissions {
	return ChannelPermissions{
		Allowed: map[string]bool{
			"local": true,
			"sdk":   true,
		},
		Claimed: map[string]bool{},
	}
}

func (p ChannelPermissions) IsAllowed(channel string) bool {
	if channel == "" {
		return true
	}
	if len(p.Allowed) == 0 {
		return true
	}
	return p.Allowed[channel]
}

func (p *ChannelPermissions) Claim(channel string) bool {
	if channel == "" {
		return true
	}
	if p == nil || !p.IsAllowed(channel) {
		return false
	}
	if p.Claimed == nil {
		p.Claimed = map[string]bool{}
	}
	if p.Claimed[channel] {
		return false
	}
	p.Claimed[channel] = true
	return true
}

func (p *ChannelPermissions) Release(channel string) {
	if p == nil || channel == "" || p.Claimed == nil {
		return
	}
	delete(p.Claimed, channel)
}

func CreateChannelPermissionCallbacks(channel string, perms *ChannelPermissions) ChannelPermissionCallbacks {
	return ChannelPermissionCallbacks{
		channel: channel,
		perms:   perms,
	}
}

func (c ChannelPermissionCallbacks) Claim() bool {
	if c.perms == nil {
		return true
	}
	return c.perms.Claim(c.channel)
}

func (c ChannelPermissionCallbacks) Release() {
	if c.perms == nil {
		return
	}
	c.perms.Release(c.channel)
}
