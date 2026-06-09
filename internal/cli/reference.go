package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/applause/bachkator/docs"
)

func runReference(args []string, stdout io.Writer, _ io.Writer) error {
	if len(args) == 0 {
		headings, err := docs.Headings()
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(stdout, docs.FormatHeadings(headings))
		return err
	}
	query := strings.Join(args, " ")
	sections, err := docs.Search(query)
	if err != nil {
		return err
	}
	if len(sections) == 0 {
		return fmt.Errorf("no reference entries matched %q", query)
	}
	_, err = fmt.Fprint(stdout, docs.FormatSections(sections))
	return err
}
