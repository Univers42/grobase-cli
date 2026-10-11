package cli

import (
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/Univers42/grobase-cli/internal/env"
)

func newVersionCmd(e *env.Env, info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return e.Printer().Print(map[string]string{
				"version": info.Version, "commit": info.Commit, "date": info.Date,
				"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
			})
		},
	}
}

func newDocsCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "docs DIR",
		Short:  "Generate the markdown command reference into DIR",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			root.DisableAutoGenTag = true
			return doc.GenMarkdownTree(root, args[0])
		},
	}
}
