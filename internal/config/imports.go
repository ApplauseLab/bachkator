package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/applauselab/bachkator/internal/dag"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type importDecl struct {
	path      string
	rangeInfo hcl.Range
}

type loadedBachfile struct {
	path   string
	body   *hclsyntax.Body
	blocks hclsyntax.Blocks
}

func loadBachfile(path string) (*hcl.File, error) {
	parser := hclparse.NewParser()
	loader := &importLoader{
		parser: parser,
		seen:   map[string]bool{},
		graph:  dag.New[string, string](),
	}
	loaded, err := loader.load(path, nil, true)
	if err != nil {
		return nil, err
	}
	return &hcl.File{Body: loaded.body}, nil
}

type importLoader struct {
	parser *hclparse.Parser
	seen   map[string]bool
	graph  *dag.Graph[string, string]
}

func (l *importLoader) load(path string, stack []string, root bool) (*loadedBachfile, error) {
	canonical, err := canonicalImportPath(path)
	if err != nil {
		return nil, err
	}
	if cycleStart := indexString(stack, canonical); cycleStart >= 0 {
		cycle := append(append([]string(nil), stack[cycleStart:]...), canonical)
		return nil, fmt.Errorf("import cycle: %s", strings.Join(cycle, " -> "))
	}
	if l.seen[canonical] {
		return &loadedBachfile{path: canonical}, nil
	}
	l.seen[canonical] = true
	l.graph.AddVertex(dag.Vertex[string]{ID: canonical, Kind: "bachfile"})

	source, err := os.ReadFile(canonical)
	if err != nil {
		return nil, fmt.Errorf("import %s: read %s: %w", importStack(stack), canonical, err)
	}
	imports, parseableSource, err := collectImports(canonical, source)
	if err != nil {
		return nil, err
	}
	file, diags := l.parser.ParseHCL(parseableSource, canonical)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse %s: %s", canonical, diags.Error())
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("parse %s: unsupported HCL body", canonical)
	}
	if !root && hasProjectBlock(body) {
		return nil, fmt.Errorf("%s: imported Bachfile must not declare a project block", canonical)
	}

	nextStack := append(append([]string(nil), stack...), canonical)
	importedBlocks := make(map[int]hclsyntax.Blocks, len(imports))
	for _, decl := range imports {
		importPath, err := resolveImportPath(canonical, decl)
		if err != nil {
			return nil, err
		}
		l.graph.AddEdge(canonical, importPath, "import")
		if err := l.graph.ValidateAcyclic(); err != nil {
			return nil, fmt.Errorf("import cycle: %w", err)
		}
		loaded, err := l.load(importPath, nextStack, false)
		if err != nil {
			return nil, fmt.Errorf("%s: import %q: %w", decl.rangeInfo.String(), decl.path, err)
		}
		importedBlocks[decl.rangeInfo.Start.Byte] = loaded.blocks
	}

	body.Blocks = mergeImportBlocks(body.Blocks, imports, importedBlocks)
	return &loadedBachfile{path: canonical, body: body, blocks: body.Blocks}, nil
}

func collectImports(path string, source []byte) ([]importDecl, []byte, error) {
	parseable := append([]byte(nil), source...)
	imports := []importDecl{}
	lineStart := 0
	line := 1
	depth := 0
	state := importScanState{}
	for i := 0; i <= len(source); i++ {
		if i < len(source) && source[i] != '\n' {
			continue
		}
		lineBytes := source[lineStart:i]
		trimmed := strings.TrimSpace(string(lineBytes))
		if depth == 0 && isImportLine(trimmed) {
			decl, err := parseImportLine(path, line, lineStart, lineBytes, trimmed)
			if err != nil {
				return nil, nil, err
			}
			imports = append(imports, decl)
			for j := lineStart; j < i; j++ {
				if parseable[j] != '\t' {
					parseable[j] = ' '
				}
			}
		} else {
			depth += braceDelta(string(lineBytes), &state)
			if depth < 0 {
				depth = 0
			}
		}
		lineStart = i + 1
		line++
	}
	return imports, parseable, nil
}

func parseImportLine(
	path string,
	line int,
	lineStart int,
	lineBytes []byte,
	trimmed string,
) (importDecl, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "import"))
	if !strings.HasPrefix(rest, "\"") {
		return importDecl{}, fmt.Errorf("%s:%d: import path must be a string literal", path, line)
	}
	end := closingQuote(rest)
	if end < 0 {
		return importDecl{}, invalidImportStringError(path, line)
	}
	quoted := rest[:end+1]
	importPath, err := strconv.Unquote(quoted)
	if err != nil {
		return importDecl{}, invalidImportStringError(path, line)
	}
	if strings.Contains(importPath, "${") || strings.Contains(importPath, "%{") {
		return importDecl{}, fmt.Errorf("%s:%d: import path must be a string literal", path, line)
	}
	remaining := strings.TrimSpace(rest[end+1:])
	if hasImportTrailingContent(remaining) {
		return importDecl{}, invalidImportTrailingContentError(path, line)
	}
	column := strings.Index(string(lineBytes), "import") + 1
	start := hcl.Pos{Line: line, Column: column, Byte: lineStart + column - 1}
	endPos := hcl.Pos{Line: line, Column: len(lineBytes) + 1, Byte: lineStart + len(lineBytes)}
	return importDecl{
		path: importPath,
		rangeInfo: hcl.Range{
			Filename: path,
			Start:    start,
			End:      endPos,
		},
	}, nil
}

func invalidImportStringError(path string, line int) error {
	return fmt.Errorf("%s:%d: import path must be a valid string literal", path, line)
}

func hasImportTrailingContent(remaining string) bool {
	isComment := strings.HasPrefix(remaining, "#") || strings.HasPrefix(remaining, "//")
	return remaining != "" && !isComment
}

func invalidImportTrailingContentError(path string, line int) error {
	return fmt.Errorf("%s:%d: import declaration must contain only a path", path, line)
}

func mergeImportBlocks(
	blocks hclsyntax.Blocks,
	imports []importDecl,
	importedBlocks map[int]hclsyntax.Blocks,
) hclsyntax.Blocks {
	items := make([]struct {
		pos    int
		blocks hclsyntax.Blocks
	}, 0, len(blocks)+len(imports))
	for _, block := range blocks {
		items = append(items, struct {
			pos    int
			blocks hclsyntax.Blocks
		}{pos: block.DefRange().Start.Byte, blocks: hclsyntax.Blocks{block}})
	}
	for _, decl := range imports {
		items = append(items, struct {
			pos    int
			blocks hclsyntax.Blocks
		}{pos: decl.rangeInfo.Start.Byte, blocks: importedBlocks[decl.rangeInfo.Start.Byte]})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].pos < items[j].pos })
	merged := hclsyntax.Blocks{}
	for _, item := range items {
		merged = append(merged, item.blocks...)
	}
	return merged
}

func canonicalImportPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if realPath, err := filepath.EvalSymlinks(abs); err == nil {
		return realPath, nil
	}
	return filepath.Clean(abs), nil
}

func closingQuote(value string) int {
	escaped := false
	for i := 1; i < len(value); i++ {
		if escaped {
			escaped = false
			continue
		}
		switch value[i] {
		case '\\':
			escaped = true
		case '"':
			return i
		}
	}
	return -1
}

type importScanState struct {
	inBlockComment bool
}

func braceDelta(line string, state *importScanState) int {
	delta := 0
	inString := false
	escaped := false
	for i := 0; i < len(line); i++ {
		if state.inBlockComment {
			if i+1 < len(line) && line[i] == '*' && line[i+1] == '/' {
				state.inBlockComment = false
				i++
			}
			continue
		}
		if escaped {
			escaped = false
			continue
		}
		if line[i] == '\\' && inString {
			escaped = true
			continue
		}
		if line[i] == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if line[i] == '#' {
			break
		}
		if i+1 < len(line) && line[i] == '/' && line[i+1] == '/' {
			break
		}
		if i+1 < len(line) && line[i] == '/' && line[i+1] == '*' {
			state.inBlockComment = true
			i++
			continue
		}
		switch line[i] {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

func indexString(values []string, value string) int {
	for i, candidate := range values {
		if candidate == value {
			return i
		}
	}
	return -1
}

func importStack(stack []string) string {
	if len(stack) == 0 {
		return "root"
	}
	return strings.Join(stack, " -> ")
}

func isImportLine(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "import") {
		return false
	}
	if len(trimmed) == len("import") {
		return true
	}
	next := trimmed[len("import")]
	return next == ' ' || next == '\t' || next == '"'
}

func resolveImportPath(importer string, decl importDecl) (string, error) {
	if decl.path == "" {
		return "", fmt.Errorf(
			"%s:%d: import path must not be empty",
			importer,
			decl.rangeInfo.Start.Line,
		)
	}
	if filepath.IsAbs(decl.path) || strings.Contains(decl.path, "://") {
		return "", fmt.Errorf(
			"%s:%d: import path must be a local relative path",
			importer,
			decl.rangeInfo.Start.Line,
		)
	}
	if strings.ContainsAny(decl.path, "*?[") {
		return "", fmt.Errorf(
			"%s:%d: glob imports are not supported",
			importer,
			decl.rangeInfo.Start.Line,
		)
	}
	resolved := filepath.Join(filepath.Dir(importer), decl.path)
	canonical, err := canonicalImportPath(resolved)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(canonical); err != nil {
		return "", fmt.Errorf(
			"%s:%d: import %q: %w",
			importer,
			decl.rangeInfo.Start.Line,
			decl.path,
			err,
		)
	}
	return canonical, nil
}

func hasProjectBlock(body hcl.Body) bool {
	content, _, _ := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "project", LabelNames: []string{"name"}}},
	})
	return len(content.Blocks) > 0
}
