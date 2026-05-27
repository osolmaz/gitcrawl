package cli

import (
	"context"
	"errors"
	"fmt"
)

var errLocalGHUnsupported = errors.New("local gh shim unsupported")

func (a *App) runGHShim(_ context.Context, _ []string) error {
	_, _ = fmt.Fprintln(a.Stderr, "gitcrawl gh moved to octopool.")
	_, _ = fmt.Fprintln(a.Stderr, "Run: octopool login")
	_, _ = fmt.Fprintln(a.Stderr, "Then use: octopool gh ... or symlink octopool as gh.")
	return usageErr(errors.New("gitcrawl gh moved to octopool"))
}

func localGHUnsupported(err error) error {
	if err == nil {
		return errLocalGHUnsupported
	}
	return fmt.Errorf("%w: %v", errLocalGHUnsupported, err)
}
