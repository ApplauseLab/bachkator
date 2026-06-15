package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
)

func runAffected(
	ctx context.Context,
	project *Project,
	deps Dependencies,
	paths []string,
	stdout io.Writer,
) error {
	if deps.AffectedTargets == nil {
		return fmt.Errorf("affected service is not configured")
	}
	affected, err := deps.AffectedTargets(ctx, project, paths)
	if err != nil {
		return err
	}
	for _, target := range affected {
		matches := target.Matches
		if len(matches) > 3 {
			matches = matches[:3]
		}
		if _, err := fmt.Fprintf(
			stdout,
			"%s %d %s\n",
			target.Name,
			len(target.Matches),
			strings.Join(matches, ", "),
		); err != nil {
			return err
		}
	}
	return nil
}
