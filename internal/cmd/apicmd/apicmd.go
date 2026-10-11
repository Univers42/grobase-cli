// Package apicmd implements `grobase api`: a raw, authenticated call to any
// endpoint of the current context, like `gh api`.
package apicmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// New returns the `api` command.
func New(e *env.Env) *cobra.Command {
	var f apiFlags
	cmd := &cobra.Command{
		Use:   "api METHOD PATH",
		Short: "Call any endpoint of the current context",
		Long: `METHOD is sent upper-cased and PATH must start with "/".

Fields come from -f (always a string) and -F (true/false/null and numbers are
typed, anything else stays a string). A GET carries them in the query string,
every other method sends them as a JSON body. --input FILE sends a raw body
("-" is stdin) and wins over fields.

--paginate walks the results: the Link header with rel="next" is followed, and a
PostgREST-style Content-Range answer ("0-24/100") is continued by re-sending the
request with the next offset. Array answers are merged into one result.
--include prints the status line and headers to stderr, --raw writes the body
exactly as received.`,
		Args: exitcode.Args(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := parseMode(f.mode)
			if err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			return call(cmd.Context(), e, cl, f, m, strings.ToUpper(args[0]), args[1])
		},
	}
	fl := cmd.Flags()
	fl.StringArrayVarP(&f.fields, "raw-field", "f", nil, "string field, key=value (repeatable)")
	fl.StringArrayVarP(&f.typed, "field", "F", nil, "typed field, key=value: true/false/null/number, else string (repeatable)")
	fl.StringArrayVarP(&f.headers, "header", "H", nil, "extra header, 'Name: value' (repeatable)")
	fl.StringVar(&f.input, "input", "", "raw body from FILE (\"-\" means stdin); sent as application/json when it parses, else octet-stream, unless -H sets Content-Type")
	fl.BoolVar(&f.paginate, "paginate", false, "follow every page until the results are exhausted")
	fl.IntVar(&f.maxPages, "max-pages", 100, "stop after that many pages (with --paginate)")
	fl.BoolVar(&f.include, "include", false, "print the status line and headers to stderr")
	fl.BoolVar(&f.raw, "raw", false, "print the response body exactly as received")
	fl.StringVar(&f.mode, "mode", "auto", "credential to send: auto, user, service, apikey, anon or none")
	return cmd
}

// apiFlags carries every option of `api`.
type apiFlags struct {
	fields, typed, headers []string
	input                  string
	paginate               bool
	maxPages               int
	include, raw           bool
	mode                   string
}

// call performs one request, or the whole paginated walk, and prints the answer.
func call(ctx context.Context, e *env.Env, cl *httpx.Client, f apiFlags, mode httpx.Mode, method, path string) error {
	req, err := f.request(e, method, path, mode)
	if err != nil {
		return err
	}
	pages, err := walk(ctx, cl, e, req, f)
	if err != nil {
		return err
	}
	return emit(e, pages, f)
}
