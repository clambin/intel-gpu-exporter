//go:build linux || darwin

package collector

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"testing"
)

func TestRunner(t *testing.T) {
	//l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := Runner{logger: l}

	// valid command
	ctx := context.Background()
	stdout, err := r.Start(ctx, []string{"sh", "-c", "echo hello world; sleep 60"})
	require.NoError(t, err)
	line := make([]byte, 1024)
	assert.True(t, r.Running())
	n, err := stdout.Read(line)
	assert.NoError(t, err)
	assert.Equal(t, "hello world\n", string(line[:n]))
	r.Stop()

	// invalid command
	_, err = r.Start(ctx, []string{"not a command"})
	assert.Error(t, err)
	assert.False(t, r.Running())
}
