package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/applauselab/bachkator/docs"
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
	assets, assetErr := docs.SearchAssets(query)
	if assetErr != nil {
		return assetErr
	}
	if len(assets) > 0 {
		_, err := fmt.Fprint(stdout, docs.FormatAssets(assets))
		return err
	}
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
