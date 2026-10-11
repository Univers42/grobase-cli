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

// subID identifies this command's single subscription to the gateway.
const subID = "cli-1"

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
	fl.StringVar(&f.event, "event", "", "only receive events with this label")
	fl.IntVar(&f.count, "count", 0, "stop after this many messages (0 = no limit)")
	fl.DurationVar(&f.timeout, "timeout", 0, "stop after this long (0 = no limit)")
	return cmd
}

// runSubscribe opens the socket, subscribes to the channel and prints events
// until --count of them are printed, the deadline, a peer close or an
// interrupt.
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
	conn, err := dial(ctx, e, f.timeout)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	if err := writeJSON(ctx, conn, subscribeFrame{Type: "SUBSCRIBE", SubID: subID, Topic: channel}); err != nil {
		return err
	}
	if err := awaitSubscribed(ctx, e, conn, channel); err != nil {
		return err
	}
	return stream(ctx, e, conn, f)
}

// subscribeFrame opens one subscription on the gateway.
type subscribeFrame struct {
	Type  string `json:"type"`
	SubID string `json:"sub_id"`
	Topic string `json:"topic"`
}

// awaitSubscribed waits for the gateway to confirm the subscription, reporting
// the confirmation on stderr when --debug is on.
func awaitSubscribed(ctx context.Context, e *env.Env, conn *websocket.Conn, channel string) error {
	for {
		typ, b, err := conn.Read(ctx)
		if err != nil {
			return e.Redact.Err(fmt.Errorf("read SUBSCRIBED: %w", err))
		}
		if typ != websocket.MessageText {
			continue
		}
		var f struct {
			envelope
			SubID string `json:"sub_id"`
			Seq   int    `json:"seq"`
		}
		if err := json.Unmarshal(b, &f); err != nil {
			return e.Redact.Err(fmt.Errorf("decode SUBSCRIBED frame: %w", err))
		}
		switch f.Type {
		case "SUBSCRIBED":
			if e.Flags.Debug {
				_, _ = fmt.Fprintf(e.Stderr, "subscribed to %s (seq %d)\n", channel, f.Seq)
			}
			return nil
		case "ERROR":
			return e.Redact.Err(fmt.Errorf("subscription refused (%s): %s", f.Code, f.Message))
		}
	}
}

// stream prints every EVENT of the subscription on its own line until count
// events are printed, the deadline, a peer close or an interrupt. The pretty
// printers cannot be used here: a stream must stay one line per message.
func stream(ctx context.Context, e *env.Env, conn *websocket.Conn, f subscribeFlags) error {
	enc := json.NewEncoder(e.Stdout)
	enc.SetEscapeHTML(false)
	for printed := 0; ; {
		typ, b, err := conn.Read(ctx)
		if err != nil {
			return streamEnded(err)
		}
		if typ != websocket.MessageText {
			continue
		}
		line, err := eventLine(e, b, f.event)
		if err != nil {
			return err
		}
		if line == nil {
			continue
		}
		if err := enc.Encode(line); err != nil {
			return err
		}
		printed++
		if f.count > 0 && printed >= f.count {
			return nil
		}
	}
}

// eventLine converts one gateway frame into the line to print; it returns nil
// for frames the command does not print (PONG, unknown types) or events the
// --event filter drops.
func eventLine(e *env.Env, b []byte, want string) (*eventOut, error) {
	var f struct {
		envelope
		Event eventPayload `json:"event"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("decode gateway frame: %w", err)
	}
	switch f.Type {
	case "EVENT":
	case "ERROR":
		return nil, e.Redact.Err(fmt.Errorf("gateway error (%s): %s", f.Code, f.Message))
	default:
		return nil, nil
	}
	label := f.Event.label()
	if want != "" && label != want {
		return nil, nil
	}
	return &eventOut{Topic: f.Event.Topic, Event: label, Payload: f.Event.Payload}, nil
}

// eventPayload is the event object of an EVENT frame.
type eventPayload struct {
	Topic     string          `json:"topic"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

// label returns the name an event is printed and filtered by: the event a
// broadcast carries in its payload, else the server's event type.
func (p eventPayload) label() string {
	var b struct {
		Event string `json:"event"`
	}
	if json.Unmarshal(p.Payload, &b) == nil && b.Event != "" {
		return b.Event
	}
	return p.EventType
}

// eventOut is one printed event: topic, label and payload, in that order.
type eventOut struct {
	Topic   string          `json:"topic"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
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
