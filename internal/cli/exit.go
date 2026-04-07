package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
)

func Exit(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		os.Exit(130)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
