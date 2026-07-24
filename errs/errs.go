package errs

import (
	"errors"
)

var (
	ErrConfigTypeUnsupported error = errors.New(
		"The configuration type is unsupported, use a file:// URL or a path",
	)

	ErrNoAccounts error = errors.New(
		"No accounts are configured, add at least one [[account]] to the " +
			"configuration",
	)

	ErrAccountNameRequired error = errors.New(
		"Every account needs a name",
	)

	ErrAccountEndpointRequired error = errors.New(
		"Every account needs an endpoint",
	)

	ErrKeyNotFound error = errors.New(
		"Key not found",
	)
)
