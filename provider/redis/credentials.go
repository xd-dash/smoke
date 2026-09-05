package redisprovider

// Credentials are Redis AUTH credentials applied to a Target before dialing.
// An empty Username with a non-empty Password uses Redis' default user / legacy
// password-only AUTH form. A non-empty Username uses ACL username+password AUTH.
type Credentials struct {
	Username string
	Password string
}

func (c Credentials) Apply(target Target) Target {
	target.Username = c.Username
	target.Password = c.Password
	return target
}
