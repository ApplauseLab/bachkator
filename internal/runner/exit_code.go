package runner

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

func exitCodeFromError(err error) *int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		fields := strings.Fields(err.Error())
		if len(fields) >= 3 && fields[len(fields)-2] == "status" {
			if code, parseErr := strconv.Atoi(fields[len(fields)-1]); parseErr == nil {
				return &code
			}
		}
		return nil
	}
	code := exitErr.ExitCode()
	return &code
}
