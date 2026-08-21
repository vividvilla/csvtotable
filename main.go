package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	"github.com/xuri/excelize/v2"
	"github.com/yuin/goldmark"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

//go:embed table/dist/csvtotable-table.min.js
var tableJS string

//go:embed table/dist/csvtotable-table.min.css
var tableCSS string

var version = "dev"

const stdioArgument = "\x00"

// themes must match the [data-theme] blocks in table/src/table.css. "auto"
// pins nothing, leaving the stylesheet to follow the system.
var themes = []string{"auto", "default", "dark", "nord", "gruvbox", "solarized"}

type options struct {
	InputFiles      []string
	OutputFile      string
	Title           string
	TitleHTML       string
	Description     string
	DescriptionHTML string
	Delimiter       string
	Quote           string
	PageSize        int
	Overwrite       bool
	Serve           bool
	Height          string
	Pagination      bool
	VirtualScroll   int64
	NoHeader        bool
	ExportEnabled   bool
	ExportOptions   []string
	PreserveSort    bool
	Encoding        string
	ColumnFilters   bool
	Theme           string
}

type tableOptions struct {
	PageSize      int      `json:"displayLength"`
	Height        string   `json:"height"`
	Pagination    bool     `json:"pagination"`
	VirtualScroll int64    `json:"virtualScroll"`
	PreserveSort  bool     `json:"preserveSort"`
	ExportEnabled bool     `json:"exportEnabled"`
	ExportOptions []string `json:"exportOptions"`
	ColumnFilters bool     `json:"columnFilters"`
}

func main() {
	command := newCommand(run)
	if err := command.Run(context.Background(), protectStdioArgs(command, os.Args)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cli.HelpFlag = &cli.BoolFlag{Name: "help", Usage: "Show help", HideDefault: true, Local: true}
	cli.VersionFlag = &cli.BoolFlag{Name: "version", Usage: "Print version", HideDefault: true, Local: true}
	cli.VersionPrinter = func(command *cli.Command) {
		fmt.Fprintf(command.Root().Writer, "csvtotable %s\n", command.Version)
	}
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return version
}

func parseArgs(args []string) (options, error) {
	var parsed options
	command := newCommand(func(cli options) error {
		parsed = cli
		return nil
	})
	command.Writer = io.Discard
	command.ErrWriter = io.Discard
	err := command.Run(context.Background(), protectStdioArgs(command, append([]string{"csvtotable"}, args...)))
	return parsed, err
}

func protectStdioArgs(command *cli.Command, args []string) []string {
	valueFlags := map[string]bool{}
	for _, flag := range command.Flags {
		if documented, ok := flag.(cli.DocGenerationFlag); ok && documented.TakesValue() {
			for _, name := range flag.Names() {
				valueFlags[name] = true
			}
		}
	}
	protected := append([]string(nil), args...)
	expectValue, flagsEnded := false, false
	for i := 1; i < len(protected); i++ {
		if expectValue {
			expectValue = false
			continue
		}
		if protected[i] == "-" {
			protected[i] = stdioArgument
			continue
		}
		if protected[i] == "--" {
			flagsEnded = true
			continue
		}
		if flagsEnded {
			continue
		}
		if !strings.HasPrefix(protected[i], "-") {
			continue
		}
		name := strings.TrimLeft(protected[i], "-")
		if before, _, found := strings.Cut(name, "="); found {
			name = before
		} else {
			expectValue = valueFlags[name]
		}
	}
	return protected
}

func newCommand(action func(options) error) *cli.Command {
	var parsed options
	var disablePagination, disableExport, noColumnFilters bool
	return &cli.Command{
		Name:                      "csvtotable",
		Usage:                     "Convert CSV files into searchable, sortable HTML tables",
		UsageText:                 "csvtotable [options] INPUT... OUTPUT_FILE\n   csvtotable [options] --serve INPUT...",
		Version:                   buildVersion(),
		DisableSliceFlagSeparator: true,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "title", Aliases: []string{"caption", "c"}, Usage: "Page heading, as Markdown", Destination: &parsed.Title},
			&cli.StringFlag{Name: "title-html", Aliases: []string{"th"}, Usage: "Page heading, as raw HTML", Destination: &parsed.TitleHTML},
			&cli.StringFlag{Name: "description", Aliases: []string{"desc"}, Usage: "Text below the heading, as Markdown; @FILE reads a file", Destination: &parsed.Description},
			&cli.StringFlag{Name: "description-html", Aliases: []string{"dh"}, Usage: "Text below the heading, as raw HTML; @FILE reads a file", Destination: &parsed.DescriptionHTML},
			&cli.StringFlag{Name: "delimiter", Aliases: []string{"d"}, Value: ",", Usage: "CSV delimiter", Destination: &parsed.Delimiter},
			&cli.StringFlag{Name: "quotechar", Aliases: []string{"q"}, Value: "\"", Usage: "CSV quote character", Destination: &parsed.Quote},
			&cli.IntFlag{Name: "page-size", Aliases: []string{"display-length", "dl"}, Value: -1, Usage: "Rows per page; -1 shows all rows", Destination: &parsed.PageSize},
			&cli.BoolFlag{Name: "overwrite", Aliases: []string{"o"}, Usage: "Overwrite an existing output file", Destination: &parsed.Overwrite},
			&cli.BoolFlag{Name: "serve", Aliases: []string{"s"}, Usage: "Open a temporary result in the default browser", Destination: &parsed.Serve},
			&cli.StringFlag{Name: "height", Aliases: []string{"h"}, Usage: "Table height in px or viewport %", Destination: &parsed.Height},
			&cli.BoolFlag{Name: "pagination", Aliases: []string{"p"}, Usage: "Disable pagination", Destination: &disablePagination},
			&cli.Int64Flag{Name: "virtual-scroll", Aliases: []string{"vs"}, Value: 1000, Usage: "Virtual-scroll row threshold", Destination: &parsed.VirtualScroll},
			&cli.BoolFlag{Name: "no-header", Aliases: []string{"nh"}, Usage: "Generate column names instead of using the first row", Destination: &parsed.NoHeader},
			&cli.BoolFlag{Name: "export", Aliases: []string{"e"}, Usage: "Disable export buttons", Destination: &disableExport},
			&cli.StringSliceFlag{Name: "export-options", Aliases: []string{"eo"}, Usage: "Toolbar button: copy, csv, json, print, or colvis; may be repeated", Destination: &parsed.ExportOptions},
			&cli.BoolFlag{Name: "preserve-sort", Aliases: []string{"ps"}, Usage: "Preserve input row order", Destination: &parsed.PreserveSort},
			&cli.StringFlag{Name: "encoding", Usage: "Input character encoding", Destination: &parsed.Encoding},
			&cli.StringFlag{Name: "theme", Value: "auto", Usage: "Colour theme: " + strings.Join(themes, ", "), Destination: &parsed.Theme},
			&cli.BoolFlag{Name: "no-column-filters", Aliases: []string{"ncf"}, Usage: "Hide the per-column filter row", Destination: &noColumnFilters},
		},
		Action: func(_ context.Context, command *cli.Command) error {
			parsed.Pagination = !disablePagination
			parsed.ExportEnabled = !disableExport
			parsed.ColumnFilters = !noColumnFilters
			if !slices.Contains(themes, parsed.Theme) {
				return fmt.Errorf("invalid theme %q; choose from %s", parsed.Theme, strings.Join(themes, ", "))
			}
			if parsed.Title != "" && parsed.TitleHTML != "" {
				return errors.New("use either --title or --title-html, not both")
			}
			if parsed.Description != "" && parsed.DescriptionHTML != "" {
				return errors.New("use either --description or --description-html, not both")
			}
			for _, option := range parsed.ExportOptions {
				if !slices.Contains([]string{"copy", "csv", "json", "print", "colvis"}, option) {
					return fmt.Errorf("invalid export option %q", option)
				}
			}
			positional := command.Args().Slice()
			for i := range positional {
				if positional[i] == stdioArgument {
					positional[i] = "-"
				}
			}
			if len(positional) == 0 {
				return cli.ShowAppHelp(command)
			}
			if parsed.Serve {
				parsed.InputFiles = positional
			} else if len(positional) < 2 {
				return errors.New("missing argument \"output_file\"")
			} else {
				parsed.InputFiles = positional[:len(positional)-1]
				parsed.OutputFile = positional[len(positional)-1]
			}
			stdinInputs := 0
			for _, input := range parsed.InputFiles {
				if input == "-" {
					stdinInputs++
				}
			}
			if stdinInputs > 1 {
				return errors.New("standard input may only be used once")
			}
			return action(parsed)
		},
	}
}

func run(cli options) error {
	if cli.Serve {
		temporary, err := os.CreateTemp("", "csvtotable-*.html")
		if err != nil {
			return err
		}
		name := temporary.Name()
		defer os.Remove(name)
		if err := convert(cli, temporary); err != nil {
			temporary.Close()
			return err
		}
		if err := temporary.Close(); err != nil {
			return err
		}
		if err := openBrowser(name); err != nil {
			return err
		}
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)
		defer signal.Stop(interrupt)
		<-interrupt
		return nil
	}

	if cli.OutputFile == "-" {
		return convert(cli, os.Stdout)
	}
	if _, err := os.Stat(cli.OutputFile); err == nil && !cli.Overwrite {
		if slices.Contains(cli.InputFiles, "-") {
			return errors.New("output file exists; use --overwrite when reading from standard input")
		}
		overwrite, err := promptOverwrite(cli.OutputFile, os.Stdin)
		if err != nil {
			return err
		}
		if !overwrite {
			return errors.New("aborted")
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	target, err := resolveOutputPath(cli.OutputFile)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o666)
	removeTarget := false
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	} else {
		placeholder, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err != nil {
			return err
		}
		info, statErr := placeholder.Stat()
		closeErr := placeholder.Close()
		if statErr != nil {
			os.Remove(target)
			return statErr
		}
		if closeErr != nil {
			os.Remove(target)
			return closeErr
		}
		mode = info.Mode().Perm()
		removeTarget = true
		defer func() {
			if removeTarget {
				os.Remove(target)
			}
		}()
	}

	parent := filepath.Dir(target)
	temporary, err := os.CreateTemp(parent, ".csvtotable-*.html")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if err := convert(cli, temporary); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return err
	}
	removeTarget = false
	fmt.Printf("File converted successfully: %s\n", cli.OutputFile)
	return nil
}

func resolveOutputPath(path string) (string, error) {
	for range 255 {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}
		link, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		path = filepath.Clean(link)
	}
	return "", errors.New("too many symbolic links in output path")
}

func openBrowser(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	address := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String()
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		address = (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(absolute)}).String()
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	case "darwin":
		command = exec.Command("open", address)
	default:
		command = exec.Command("xdg-open", address)
	}
	return command.Run()
}

func promptOverwrite(path string, input io.Reader) (bool, error) {
	fmt.Fprintf(os.Stderr, "File (%s) already exists. Do you want to overwrite? (y/n): ", path)
	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer = strings.TrimSpace(answer)
	return answer == "y" || answer == "Y", nil
}

func convert(cli options, destination io.Writer) error {
	delimiter, err := csvCharacter(cli.Delimiter, "delimiter")
	if err != nil {
		return err
	}
	quote, err := csvCharacter(cli.Quote, "quotechar")
	if err != nil {
		return err
	}
	if delimiter == quote {
		return errors.New("delimiter and quotechar must differ")
	}
	if len(cli.InputFiles) == 0 {
		return errors.New("missing argument \"input_file\"")
	}

	heading, err := headingMarkup(cli)
	if err != nil {
		return err
	}
	description, err := descriptionMarkup(cli)
	if err != nil {
		return err
	}
	themeAttribute := ""
	if cli.Theme != "" && cli.Theme != "auto" {
		themeAttribute = ` data-theme="` + html.EscapeString(cli.Theme) + `"`
	}
	themePicker := &strings.Builder{}
	themePicker.WriteString(`<select class="csvtotable-theme" id="csvtotable-theme" aria-label="Colour theme">`)
	for _, name := range themes {
		selected := ""
		if name == cli.Theme || (cli.Theme == "" && name == "auto") {
			selected = " selected"
		}
		fmt.Fprintf(themePicker, `<option value="%s"%s>%s</option>`, name, selected, strings.ToUpper(name[:1])+name[1:])
	}
	themePicker.WriteString("</select>")

	title := plainText(heading)
	if title == "" {
		title = "Table"
	}
	headerHTML := ""
	tableLabel := ` aria-label="Table"`
	if heading != "" {
		headerHTML = "<h1 class=\"csvtotable-title\" id=\"csvtotable-title\">" + heading + "</h1>\n"
		tableLabel = ` aria-labelledby="csvtotable-title"`
	}
	if description != "" {
		headerHTML += "<div class=\"csvtotable-description\">" + description + "</div>\n"
	}
	if headerHTML != "" {
		headerHTML = "<header class=\"csvtotable-header\">\n" + headerHTML + "</header>\n"
	}
	height := cli.Height
	if height == "" {
		height = "auto"
	}
	if value, found := strings.CutSuffix(height, "%"); found {
		height = value + "vh"
	}
	settings := tableOptions{
		PageSize:      cli.PageSize,
		Height:        height,
		Pagination:    cli.Pagination,
		VirtualScroll: cli.VirtualScroll,
		PreserveSort:  cli.PreserveSort,
		ExportEnabled: cli.ExportEnabled,
		ExportOptions: cli.ExportOptions,
		ColumnFilters: cli.ColumnFilters,
	}

	optionsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	output := bufio.NewWriter(destination)
	headers := []string{}
	expectedColumns := -1
	started := false
	needsComma := false
	startOutput := func() error {
		headersJSON, err := json.Marshal(headers)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "<!doctype html>\n<html lang=\"en\"%s>\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<title>%s</title>\n<style>%s</style>\n</head>\n<body>\n<main class=\"csvtotable\">\n%s%s\n<table class=\"csvtotable-table\" id=\"csvtotable-table\"%s></table>\n</main>\n<script id=\"csvtotable-data\" type=\"application/json\">{\"headers\":%s,\"rows\":[", themeAttribute, html.EscapeString(title), strings.ReplaceAll(tableCSS, "</style", "<\\/style"), headerHTML, themePicker.String(), tableLabel, headersJSON)
		started = err == nil
		return err
	}
	writeRow := func(row []string) error {
		encoded, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if needsComma {
			if err := output.WriteByte(','); err != nil {
				return err
			}
		}
		if _, err := output.Write(encoded); err != nil {
			return err
		}
		needsComma = true
		return nil
	}
	strict := len(cli.InputFiles) > 1
	for inputIndex, source := range cli.InputFiles {
		input, err := openInput(source)
		if err != nil {
			return fmt.Errorf("input %q: %w", source, err)
		}
		reader, err := newRowReader(input, cli.Encoding, delimiter, quote)
		if err != nil {
			input.Close()
			return fmt.Errorf("input %q: %w", source, err)
		}
		rowNumber := 0
		readErr := func() error {
			for {
				record, err := reader.Read()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return err
				}
				rowNumber++
				if rowNumber == 1 && !cli.NoHeader {
					if inputIndex == 0 {
						headers = record
						expectedColumns = len(record)
						if err := startOutput(); err != nil {
							return err
						}
					} else if !slices.Equal(record, headers) {
						return fmt.Errorf("header %q does not match first input header %q", record, headers)
					}
					continue
				}
				if !started {
					expectedColumns = len(record)
					headers = make([]string, expectedColumns)
					for i := range headers {
						headers[i] = fmt.Sprintf("Column %d", i+1)
					}
					if err := startOutput(); err != nil {
						return err
					}
				}
				if strict && len(record) != expectedColumns {
					return fmt.Errorf("row %d has %d columns; expected %d", rowNumber, len(record), expectedColumns)
				}
				for len(record) < expectedColumns {
					record = append(record, "")
				}
				if err := writeRow(record); err != nil {
					return err
				}
			}
			if strict && !cli.NoHeader && rowNumber == 0 {
				return errors.New("missing header")
			}
			return nil
		}()
		if closer, ok := reader.(io.Closer); ok {
			closer.Close()
		}
		closeErr := input.Close()
		if readErr != nil {
			return fmt.Errorf("input %q: %w", source, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("input %q: %w", source, closeErr)
		}
	}
	if !started {
		if err := startOutput(); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(output, "]}</script>\n<script id=\"csvtotable-options\" type=\"application/json\">%s</script>\n<script>%s</script>\n<script>CsvToTable.setupTheme(\"#csvtotable-theme\");CsvToTable.createCsvTable(\"#csvtotable-table\",JSON.parse(document.getElementById(\"csvtotable-data\").textContent),JSON.parse(document.getElementById(\"csvtotable-options\").textContent));</script>\n</body>\n</html>\n", optionsJSON, strings.ReplaceAll(tableJS, "</script", "<\\/script")); err != nil {
		return err
	}
	return output.Flush()
}

// decompress transparently unwraps gzip input, detected by magic bytes so it
// works for files, URLs, and standard input alike.
func decompress(input io.ReadCloser) (io.ReadCloser, error) {
	buffered := bufio.NewReader(input)
	magic, _ := buffered.Peek(2)
	if len(magic) < 2 || magic[0] != 0x1f || magic[1] != 0x8b {
		return readCloser{buffered, input}, nil
	}
	unzipped, err := gzip.NewReader(buffered)
	if err != nil {
		input.Close()
		return nil, err
	}
	return readCloser{unzipped, input}, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}

// readText expands curl-style @FILE references in flag values.
func readText(value string) (string, error) {
	path, found := strings.CutPrefix(value, "@")
	if !found {
		return value, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// renderMarkdown converts Markdown to HTML. Goldmark drops raw HTML unless it is
// told otherwise, which is what keeps --title separate from --title-html.
func renderMarkdown(source string) (string, error) {
	var rendered bytes.Buffer
	if err := goldmark.Convert([]byte(source), &rendered); err != nil {
		return "", err
	}
	return strings.TrimSpace(rendered.String()), nil
}

// renderInlineMarkdown drops the paragraph wrapper so the result can live inside
// a heading.
func renderInlineMarkdown(source string) (string, error) {
	rendered, err := renderMarkdown(source)
	if err != nil {
		return "", err
	}
	inner, trimmed := strings.CutPrefix(rendered, "<p>")
	if inner, ok := strings.CutSuffix(inner, "</p>"); trimmed && ok && !strings.Contains(inner, "<p>") {
		return inner, nil
	}
	return rendered, nil
}

var markupTag = regexp.MustCompile(`<[^>]*>`)

// plainText reduces generated markup to the text used for the document title.
func plainText(markup string) string {
	return strings.TrimSpace(html.UnescapeString(markupTag.ReplaceAllString(markup, "")))
}

func headingMarkup(cli options) (string, error) {
	if cli.TitleHTML != "" {
		return strings.TrimSpace(cli.TitleHTML), nil
	}
	if strings.TrimSpace(cli.Title) == "" {
		return "", nil
	}
	return renderInlineMarkdown(cli.Title)
}

func descriptionMarkup(cli options) (string, error) {
	if cli.DescriptionHTML != "" {
		content, err := readText(cli.DescriptionHTML)
		if err != nil {
			return "", fmt.Errorf("description-html: %w", err)
		}
		return strings.TrimSpace(content), nil
	}
	if cli.Description == "" {
		return "", nil
	}
	content, err := readText(cli.Description)
	if err != nil {
		return "", fmt.Errorf("description: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
	return renderMarkdown(content)
}

func openInput(source string) (io.ReadCloser, error) {
	parsed, err := url.Parse(source)
	if err == nil && (strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")) {
		client := http.Client{Timeout: 2 * time.Minute}
		response, err := client.Get(source)
		if err != nil {
			return nil, err
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			response.Body.Close()
			return nil, fmt.Errorf("HTTP %s", response.Status)
		}
		return decompress(response.Body)
	}
	if strings.Contains(source, "://") {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	if source == "-" {
		return decompress(io.NopCloser(os.Stdin))
	}
	file, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	return decompress(file)
}

func csvCharacter(value, name string) (rune, error) {
	if value == "\\t" {
		value = "\t"
	}
	characters := []rune(value)
	if len(characters) != 1 || characters[0] == 0 || characters[0] == '\n' || characters[0] == '\r' || characters[0] > 127 {
		return 0, fmt.Errorf("%s must be exactly one ASCII character", name)
	}
	return characters[0], nil
}

func decodeInput(input io.Reader, label string) (io.Reader, error) {
	if label == "" {
		return transform.NewReader(input, unicode.BOMOverride(transform.Nop)), nil
	}
	encoding, err := htmlindex.Get(strings.TrimSpace(label))
	if err != nil {
		return nil, fmt.Errorf("unknown encoding: %s", label)
	}
	return transform.NewReader(input, transform.Chain(encoding.NewDecoder(), unicode.UTF8BOM.NewDecoder())), nil
}

// rowReader is the shape shared by the CSV and worksheet readers.
type rowReader interface {
	Read() ([]string, error)
}

// newRowReader picks a reader from the content itself: a zip signature means an
// Excel workbook, anything else is text to be decoded and parsed as CSV.
func newRowReader(input io.Reader, encoding string, delimiter, quote rune) (rowReader, error) {
	buffered := bufio.NewReader(input)
	if magic, _ := buffered.Peek(4); string(magic) == "PK\x03\x04" {
		return newSheetReader(buffered)
	}
	decoded, err := decodeInput(buffered, encoding)
	if err != nil {
		return nil, err
	}
	return newCSVReader(decoded, delimiter, quote), nil
}

// sheetReader streams the first worksheet of a workbook.
type sheetReader struct {
	workbook *excelize.File
	rows     *excelize.Rows
}

func newSheetReader(input io.Reader) (*sheetReader, error) {
	workbook, err := excelize.OpenReader(input)
	if err != nil {
		return nil, err
	}
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		workbook.Close()
		return nil, errors.New("workbook has no sheets")
	}
	rows, err := workbook.Rows(sheets[0])
	if err != nil {
		workbook.Close()
		return nil, err
	}
	return &sheetReader{workbook: workbook, rows: rows}, nil
}

func (reader *sheetReader) Read() ([]string, error) {
	if !reader.rows.Next() {
		if err := reader.rows.Error(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	return reader.rows.Columns()
}

func (reader *sheetReader) Close() error {
	reader.rows.Close()
	return reader.workbook.Close()
}

type csvReader struct {
	input     *bufio.Reader
	delimiter rune
	quote     rune
}

func newCSVReader(input io.Reader, delimiter, quote rune) *csvReader {
	return &csvReader{input: bufio.NewReader(input), delimiter: delimiter, quote: quote}
}

func (reader *csvReader) Read() ([]string, error) {
	fields := []string{}
	var field strings.Builder
	inQuotes, afterQuote, started := false, false, false
	for {
		character, _, err := reader.input.ReadRune()
		if errors.Is(err, io.EOF) {
			if inQuotes {
				return nil, errors.New("unterminated quoted field")
			}
			if !started && len(fields) == 0 && field.Len() == 0 {
				return nil, io.EOF
			}
			return append(fields, field.String()), nil
		}
		if err != nil {
			return nil, err
		}

		if inQuotes {
			if character != reader.quote {
				field.WriteRune(character)
				continue
			}
			next, _, err := reader.input.ReadRune()
			if errors.Is(err, io.EOF) {
				return append(fields, field.String()), nil
			}
			if err != nil {
				return nil, err
			}
			if next == reader.quote {
				field.WriteRune(reader.quote)
				continue
			}
			if err := reader.input.UnreadRune(); err != nil {
				return nil, err
			}
			inQuotes, afterQuote = false, true
			continue
		}

		if afterQuote {
			switch character {
			case reader.delimiter:
				fields = append(fields, field.String())
				field.Reset()
				afterQuote = false
			case '\n':
				return append(fields, field.String()), nil
			case '\r':
				if err := reader.consumeLF(); err != nil {
					return nil, err
				}
				return append(fields, field.String()), nil
			default:
				field.WriteRune(character)
				afterQuote = false
				started = true
			}
			continue
		}

		switch character {
		case reader.delimiter:
			fields = append(fields, field.String())
			field.Reset()
			started = true
		case reader.quote:
			if field.Len() == 0 {
				inQuotes = true
			} else {
				field.WriteRune(character)
			}
			started = true
		case '\n':
			if !started && len(fields) == 0 && field.Len() == 0 {
				continue
			}
			return append(fields, field.String()), nil
		case '\r':
			if err := reader.consumeLF(); err != nil {
				return nil, err
			}
			if !started && len(fields) == 0 && field.Len() == 0 {
				continue
			}
			return append(fields, field.String()), nil
		default:
			field.WriteRune(character)
			started = true
		}
	}
}

func (reader *csvReader) consumeLF() error {
	next, _, err := reader.input.ReadRune()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	if next != '\n' {
		return reader.input.UnreadRune()
	}
	return nil
}
