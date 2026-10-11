package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/coder/websocket"
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

type publishFlags struct {
	event     string
	data      string
	dataStdin bool
}

func newPublishCmd(e *env.Env) *cobra.Command {
	var f publishFlags
	cmd := &cobra.Command{
		Use:   "publish CHANNEL --event EVENT (--data JSON | --data-stdin)",
		Short: "Broadcast one message on a channel",
		Long: "The payload is data, not a secret, so --data may carry it directly;\n" +
			"--data-stdin reads it from standard input instead.",
		Args: exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			channel, err := channelArg(args[0])
			if err != nil {
				return err
			}
			return runPublish(cmd.Context(), e, channel, f)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.event, "event", "", "event type of the message (required)")
	fl.StringVar(&f.data, "data", "", "message payload as JSON")
	fl.BoolVar(&f.dataStdin, "data-stdin", false, "read the JSON payload from stdin")
	return cmd
}

func runPublish(ctx context.Context, e *env.Env, channel string, f publishFlags) error {
	if strings.TrimSpace(f.event) == "" {
		return exitcode.Usagef("--event is required")
	}
	payload, err := payloadOf(e, f)
	if err != nil {
		return err
	}
	conn, err := dial(ctx, e)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	msg := map[string]any{
		"type": "broadcast", "channel": channel, "event": f.event,
		"payload": json.RawMessage(payload),
	}
	if err := writeJSON(ctx, conn, msg); err != nil {
		return err
	}
	return e.Printer().Print(map[string]any{"channel": channel, "event": f.event})
}

// payloadOf resolves --data or --data-stdin into the JSON payload to broadcast.
func payloadOf(e *env.Env, f publishFlags) (json.RawMessage, error) {
	switch {
	case f.dataStdin && f.data != "":
		return nil, exitcode.Usagef("--data and --data-stdin are mutually exclusive")
	case f.dataStdin:
		b, err := io.ReadAll(e.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		payload := json.RawMessage(bytes.TrimSpace(b))
		return payload, verifyJSON(payload)
	case f.data != "":
		payload := json.RawMessage(f.data)
		return payload, verifyJSON(payload)
	}
	return nil, exitcode.Usagef("pass the payload with --data or --data-stdin")
}

// verifyJSON rejects a payload before it is framed: a half-written object would
// otherwise reach the server as a truncated frame.
func verifyJSON(payload json.RawMessage) error {
	if !json.Valid(payload) {
		return exitcode.Usagef("the payload is not valid JSON")
	}
	return nil
}
