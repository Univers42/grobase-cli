package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/coder/websocket"
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

type subscribeFlags struct {
	event   string
	count   int
	timeout time.Duration
}

func newSubscribeCmd(e *env.Env) *cobra.Command {
	var f subscribeFlags
	cmd := &cobra.Command{
		Use:   "subscribe CHANNEL",
		Short: "Print the messages of one channel as they arrive (NDJSON)",
		Long: "One message per line on stdout, so the stream pipes into jq or a file.\n" +
			"Without --count the subscription runs until --timeout, the peer closes or the\n" +
			"process is interrupted.",
		Args: exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			channel, err := channelArg(args[0])
			if err != nil {
				return err
			}
			return runSubscribe(cmd.Context(), e, channel, f)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.event, "event", "", "only receive this event type")
	fl.IntVar(&f.count, "count", 0, "stop after this many messages (0 = no limit)")
	fl.DurationVar(&f.timeout, "timeout", 0, "stop after this long (0 = no limit)")
	return cmd
}

func runSubscribe(ctx context.Context, e *env.Env, channel string, f subscribeFlags) error {
	if f.count < 0 {
		return exitcode.Usagef("--count must not be negative")
	}
	if f.timeout < 0 {
		return exitcode.Usagef("--timeout must not be negative")
	}
	if f.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.timeout)
		defer cancel()
	}
	conn, err := dial(ctx, e)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	if err := writeJSON(ctx, conn, subscribeMessage(channel, f.event)); err != nil {
		return err
	}
	return stream(ctx, e, conn, f.count)
}

// subscribeMessage is the frame that opens a subscription; an event narrows it.
func subscribeMessage(channel, event string) map[string]any {
	msg := map[string]any{"type": "subscribe", "channel": channel}
	if event != "" {
		msg["event"] = event
	}
	return msg
}

// stream prints every incoming message on its own line. The pretty printers
// cannot be used here: a stream must stay one line per message.
func stream(ctx context.Context, e *env.Env, conn *websocket.Conn, count int) error {
	for printed := 0; ; {
		typ, b, err := conn.Read(ctx)
		if err != nil {
			return streamEnded(err)
		}
		if typ != websocket.MessageText {
			continue
		}
		if _, err := fmt.Fprintln(e.Stdout, string(b)); err != nil {
			return err
		}
		printed++
		if count > 0 && printed >= count {
			return nil
		}
	}
}

// streamEnded reports the normal ways a stream stops: the count was reached, the
// timeout fired, the peer closed the socket, or the process was interrupted.
func streamEnded(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, net.ErrClosed), errors.Is(err, io.EOF),
		websocket.CloseStatus(err) == websocket.StatusNormalClosure:
		return nil
	}
	return err
}

// writeJSON sends v as one text frame.
func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	return conn.Write(ctx, websocket.MessageText, b)
}
