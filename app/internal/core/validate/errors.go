package validate

import "errors"

// ErrUnknownCheck reports a selected code that is not a validation check.
var ErrUnknownCheck = errors.New("unknown validate check")
