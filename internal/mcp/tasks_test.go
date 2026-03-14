package mcp_test

import (
	"context"
	"testing"

	"github.com/guillaume7/loom/internal/mcp"
	"github.com/stretchr/testify/require"
)

type recordedNotification struct {
	method string
	params map[string]any
}

type recordingNotifier struct {
	notifications []recordedNotification
}

func (r *recordingNotifier) SendNotificationToClient(_ context.Context, method string, params map[string]any) error {
	r.notifications = append(r.notifications, recordedNotification{method: method, params: params})
	return nil
}

func TestTaskEmitter_Start_SendsNotification(t *testing.T) {
	notifier := &recordingNotifier{}
	emitter := mcp.NewTaskEmitter(notifier)

	err := emitter.Start(context.Background(), "loom-ci-poll-pr-42", "Poll CI", true)
	require.NoError(t, err)
	require.Len(t, notifier.notifications, 1)
	require.Equal(t, "loom/task/start", notifier.notifications[0].method)
	require.Equal(t, map[string]any{
		"id":          "loom-ci-poll-pr-42",
		"title":       "Poll CI",
		"cancellable": true,
	}, notifier.notifications[0].params)
}

func TestTaskEmitter_Progress_SendsNotification(t *testing.T) {
	notifier := &recordingNotifier{}
	emitter := mcp.NewTaskEmitter(notifier)

	err := emitter.Progress(context.Background(), "loom-ci-poll-pr-42", "CI checks still running")
	require.NoError(t, err)
	require.Len(t, notifier.notifications, 1)
	require.Equal(t, "loom/task/progress", notifier.notifications[0].method)
	require.Equal(t, map[string]any{
		"id":   "loom-ci-poll-pr-42",
		"text": "CI checks still running",
	}, notifier.notifications[0].params)
}

func TestTaskEmitter_Done_SendsNotification(t *testing.T) {
	notifier := &recordingNotifier{}
	emitter := mcp.NewTaskEmitter(notifier)

	result := map[string]any{"status": "success", "checks": 5}
	err := emitter.Done(context.Background(), "loom-ci-poll-pr-42", result)
	require.NoError(t, err)
	require.Len(t, notifier.notifications, 1)
	require.Equal(t, "loom/task/done", notifier.notifications[0].method)
	require.Equal(t, map[string]any{
		"id":     "loom-ci-poll-pr-42",
		"result": result,
	}, notifier.notifications[0].params)
}
