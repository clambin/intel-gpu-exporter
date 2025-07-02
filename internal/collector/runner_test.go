//go:build linux || darwin

package collector

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner(t *testing.T) {
	//l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(slog.DiscardHandler)
	r := runner{logger: l}

	// valid command
	ctx := context.Background()
	stdout, err := r.start(ctx, "sh", "-c", "echo hello world; sleep 60")
	require.NoError(t, err)
	line := make([]byte, 1024)
	assert.True(t, r.running())
	n, err := stdout.Read(line)
	assert.NoError(t, err)
	assert.Equal(t, "hello world\n", string(line[:n]))
	r.stop()

	// invalid command
	_, err = r.start(ctx, "not a command")
	assert.Error(t, err)
	assert.False(t, r.running())
}
