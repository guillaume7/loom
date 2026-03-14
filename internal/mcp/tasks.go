package mcp

import "context"

// TaskNotifier is the interface TaskEmitter requires.
type TaskNotifier interface {
	SendNotificationToClient(ctx context.Context, method string, params map[string]any) error
}

// TaskEmitter emits MCP task lifecycle events as JSON-RPC notifications.
type TaskEmitter struct {
	notifier TaskNotifier
}

// NewTaskEmitter creates a TaskEmitter wrapping the given notifier.
func NewTaskEmitter(notifier TaskNotifier) *TaskEmitter {
	return &TaskEmitter{notifier: notifier}
}

// Start emits a loom/task/start notification.
func (e *TaskEmitter) Start(ctx context.Context, id, title string, cancellable bool) error {
	return e.notifier.SendNotificationToClient(ctx, "loom/task/start", map[string]any{
		"id":          id,
		"title":       title,
		"cancellable": cancellable,
	})
}

// Progress emits a loom/task/progress notification.
func (e *TaskEmitter) Progress(ctx context.Context, id, text string) error {
	return e.notifier.SendNotificationToClient(ctx, "loom/task/progress", map[string]any{
		"id":   id,
		"text": text,
	})
}

// Done emits a loom/task/done notification.
func (e *TaskEmitter) Done(ctx context.Context, id string, result any) error {
	return e.notifier.SendNotificationToClient(ctx, "loom/task/done", map[string]any{
		"id":     id,
		"result": result,
	})
}

// Emitter returns the task emitter bound to the latest MCP server instance.
// Call MCPServer() first to ensure it is initialized.
func (s *Server) Emitter() *TaskEmitter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.emitter
}
