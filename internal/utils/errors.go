package utils

import "fmt"

func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func Wrap(scope string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", scope, err)
}
