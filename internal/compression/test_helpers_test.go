package compression

import (
	"context"

	"pdf-golang-zip/internal/worker"
)

type fakeRunner struct {
	handler func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error)
}

func (f *fakeRunner) Run(ctx context.Context, tool string, args ...string) (*worker.Result, error) {
	return f.RunWithOptions(ctx, worker.CommandOptions{}, tool, args...)
}

func (f *fakeRunner) RunWithOptions(
	ctx context.Context,
	opts worker.CommandOptions,
	tool string,
	args ...string,
) (*worker.Result, error) {
	return f.handler(ctx, opts, tool, args...)
}
