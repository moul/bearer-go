package bearer

import "errors"

var (
	// ErrBlockedDomain is raised when your program tries to make requests to a blacklisted domain.
	ErrBlockedDomain = errors.New("bearer: blocked domain")
)
