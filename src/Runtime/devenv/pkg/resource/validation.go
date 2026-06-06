package resource

import "errors"

var errResourceNameRequired = errors.New("resource name is required")

func validateName(name string) error {
	if name == "" {
		return errResourceNameRequired
	}
	return nil
}
