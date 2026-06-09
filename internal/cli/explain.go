package cli

import (
	"fmt"
	"io"
)

func runExplain(project *Project, deps Dependencies, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: bach explain <target>")
	}
	if deps.ExplainTarget == nil {
		return fmt.Errorf("explain service is not configured")
	}
	explanation, err := deps.ExplainTarget(project, args[0])
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, explanation)
	return err
}
