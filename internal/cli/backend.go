package cli

import (
	"io"
	"os"

	backendsqlite "github.com/applauselab/bachkator/internal/backend/sqlite"
	"github.com/spf13/cobra"
)

func newBackendCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backend",
		Short: "Run low-level Backend Provider JSON-RPC stdio entrypoints",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "sqlite",
		Short: "start the bundled SQLite Backend Provider",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return backendsqlite.Serve(cmd.Context(), os.Stdin, stdout)
		},
	})
	return cmd
}
