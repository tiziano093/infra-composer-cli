package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tiziano093/infra-composer-cli/internal/config"
)

func TestVersionCommand_TextOutput(t *testing.T) {
	t.Parallel()
	cmd := NewVersionCommand(BuildInfo{Version: "1.2.3", BuildTime: "now", GitCommit: "abc"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetContext(WithRuntime(context.Background(), Runtime{Stdout: &out, Config: config.Defaults()}))

	require.NoError(t, cmd.Execute())
	s := out.String()
	assert.Contains(t, s, "infra-composer 1.2.3")
	assert.Contains(t, s, "commit: abc")
	assert.Contains(t, s, "built: now")
}

func TestVersionCommand_JSONOutput(t *testing.T) {
	t.Parallel()
	cmd := NewVersionCommand(BuildInfo{Version: "9.9.9", BuildTime: "t", GitCommit: "c"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cfg := config.Defaults()
	cfg.OutputFormat = "json"
	cmd.SetContext(WithRuntime(context.Background(), Runtime{Stdout: &out, Config: cfg}))

	require.NoError(t, cmd.Execute())

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out.String())), &payload))
	assert.Equal(t, "9.9.9", payload["version"])
	assert.Equal(t, "c", payload["git_commit"])
	assert.NotEmpty(t, payload["go_version"])
}

func TestVersionCommand_FlagOverridesGlobalFormat(t *testing.T) {
	t.Parallel()
	cmd := NewVersionCommand(BuildInfo{Version: "1.0.0"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetContext(WithRuntime(context.Background(), Runtime{Stdout: &out, Config: config.Defaults()}))
	cmd.SetArgs([]string{"--format", "json"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), `"version": "1.0.0"`)
}
