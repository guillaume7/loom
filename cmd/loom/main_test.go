package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartCmd_PrintsStartingLoom(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"start"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.True(t, strings.Contains(buf.String(), "starting loom"))
}

func TestMCPCmd_PrintsStartingMCPServer(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"mcp"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.True(t, strings.Contains(buf.String(), "starting mcp server"))
}
