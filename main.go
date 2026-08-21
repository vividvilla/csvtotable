package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
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
	"strconv"
	"strings"
	"syscall"
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
	Address         string
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
	CSS             string
	JS              string
	Compress        bool
	Split           bool
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

// listenAddress matches the [HOST]:PORT that --serve accepts: a bare port, a
// host or IPv4 address, or a bracketed IPv6 address. Anything else after
// --serve is a positional argument.
var listenAddress = regexp.MustCompile(`^([A-Za-z0-9._-]*|\[[0-9A-Fa-f:.]+\]):[0-9]{1,5}$`)

// serveNames are the spellings of the optional-value --serve flag.
var serveNames = map[string]bool{"serve": true, "s": true}

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
		} else if serveNames[name] {
			// --serve takes an optional value, which urfave/cli has no notion
			// of. Only what looks like an address counts as one; otherwise the
			// flag is pinned empty so the next argument stays positional.
			expectValue = i+1 < len(protected) && listenAddress.MatchString(protected[i+1])
			if !expectValue {
				protected[i] += "="
			}
		} else {
			expectValue = valueFlags[name]
		}
	}
	return protected
}

func newCommand(action func(options) error) *cli.Command {
	var parsed options
	var disablePagination, disableExport, noColumnFilters, noCompress bool
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
			&cli.StringFlag{Name: "serve", Aliases: []string{"s"}, Usage: "Build the page and serve it; takes an optional [HOST]:PORT", Destination: &parsed.Address},
			&cli.StringFlag{Name: "height", Aliases: []string{"h"}, Usage: "Table height in px or viewport %", Destination: &parsed.Height},
			&cli.BoolFlag{Name: "pagination", Aliases: []string{"p"}, Usage: "Disable pagination", Destination: &disablePagination},
			&cli.Int64Flag{Name: "virtual-scroll", Aliases: []string{"vs"}, Value: 1000, Usage: "Virtual-scroll row threshold", Destination: &parsed.VirtualScroll},
			&cli.BoolFlag{Name: "no-header", Aliases: []string{"nh"}, Usage: "Generate column names instead of using the first row", Destination: &parsed.NoHeader},
			&cli.BoolFlag{Name: "export", Aliases: []string{"e"}, Usage: "Disable export buttons", Destination: &disableExport},
			&cli.StringSliceFlag{Name: "export-options", Aliases: []string{"eo"}, Usage: "Toolbar button: copy, csv, json, print, or colvis; may be repeated", Destination: &parsed.ExportOptions},
			&cli.BoolFlag{Name: "preserve-sort", Aliases: []string{"ps"}, Usage: "Preserve input row order", Destination: &parsed.PreserveSort},
			&cli.StringFlag{Name: "encoding", Usage: "Input character encoding", Destination: &parsed.Encoding},
			&cli.StringFlag{Name: "theme", Value: "auto", Usage: "Colour theme: " + strings.Join(themes, ", "), Destination: &parsed.Theme},
			&cli.StringFlag{Name: "css", Usage: "Path to a stylesheet to inline into the page", Destination: &parsed.CSS},
			&cli.StringFlag{Name: "js", Usage: "Path to a script to inline into the page", Destination: &parsed.JS},
			&cli.BoolFlag{Name: "no-column-filters", Aliases: []string{"ncf"}, Usage: "Hide the per-column filter row", Destination: &noColumnFilters},
			&cli.BoolFlag{Name: "no-compress", Usage: "Inline the frontend script uncompressed, for older browsers", Destination: &noCompress},
			&cli.BoolFlag{Name: "split", Usage: "Write OUTPUT as a directory of separate files the browser can cache", Destination: &parsed.Split},
		},
		Action: func(_ context.Context, command *cli.Command) error {
			parsed.Serve = command.IsSet("serve")
			parsed.Pagination = !disablePagination
			parsed.ExportEnabled = !disableExport
			parsed.ColumnFilters = !noColumnFilters
			parsed.Compress = !noCompress
			if !slices.Contains(themes, parsed.Theme) && parsed.CSS == "" {
				return fmt.Errorf("invalid theme %q; choose from %s, or define your own with --css", parsed.Theme, strings.Join(themes, ", "))
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
			if parsed.Split && parsed.OutputFile == "-" {
				return errors.New("--split writes a directory, so it cannot write to standard output")
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
		return serve(cli)
	}

	if cli.Split {
		index := filepath.Join(cli.OutputFile, indexFile)
		if _, err := os.Stat(index); err == nil && !cli.Overwrite {
			if slices.Contains(cli.InputFiles, "-") {
				return errors.New("output directory exists; use --overwrite when reading from standard input")
			}
			overwrite, err := promptOverwrite(index, os.Stdin)
			if err != nil {
				return err
			}
			if !overwrite {
				return errors.New("aborted")
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := convertDirectory(cli, cli.OutputFile); err != nil {
			return err
		}
		fmt.Printf("Directory written successfully: %s\n", cli.OutputFile)
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

// serve builds the page into a temporary directory and hands it to a local
// HTTP server. The flag is named --serve, and over http:// the split assets are
// fetched and cached the way a real deployment would do it.
func serve(cli options) error {
	directory, err := os.MkdirTemp("", "csvtotable-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)

	if cli.Split {
		if err := convertDirectory(cli, directory); err != nil {
			return err
		}
	} else {
		page, err := os.Create(filepath.Join(directory, indexFile))
		if err != nil {
			return err
		}
		if err := convert(cli, page); err != nil {
			page.Close()
			return err
		}
		if err := page.Close(); err != nil {
			return err
		}
	}

	bind, err := listenTarget(cli.Address)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: previewHandler(directory)}
	go server.Serve(listener)

	address := "http://" + browsableAddress(listener.Addr()) + "/"
	fmt.Printf("Serving %s — press Ctrl-C to stop\n", address)
	if !loopbackOnly(bind) {
		fmt.Fprintf(os.Stderr, "Warning: %s is reachable from the network, and the page contains your data\n", bind)
	}
	// A desktop without a browser handler is no reason to stop serving; the
	// address is already on screen for the user to open themselves.
	if err := openBrowser(address); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open a browser: %v\n", err)
	}
	interrupt := make(chan os.Signal, 1)
	// SIGTERM as well as Ctrl-C: a closed terminal or a supervisor stopping the
	// process would otherwise skip the cleanup and strand the temporary
	// directory, which in --split mode holds the whole frontend.
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	<-interrupt

	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

// listenTarget turns the optional --serve value into an address to bind. An
// empty value means a random loopback port; a value with no host means
// loopback too, so that exposing the data on the network stays something you
// have to write out in full.
func listenTarget(address string) (string, error) {
	if strings.TrimSpace(address) == "" {
		return "127.0.0.1:0", nil
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("--serve %q: expected [HOST]:PORT", address)
	}
	if number, err := strconv.Atoi(port); err != nil || number < 0 || number > 65535 {
		return "", fmt.Errorf("--serve %q: %q is not a port", address, port)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}

// loopbackOnly reports whether a bind address keeps the preview on this
// machine. A hostname that is not plainly loopback counts as exposed.
func loopbackOnly(bind string) bool {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		return false
	}
	if address := net.ParseIP(host); address != nil {
		return address.IsLoopback()
	}
	return host == "localhost"
}

// browsableAddress swaps an unspecified bind for a host a browser can open.
func browsableAddress(bound net.Addr) string {
	address, ok := bound.(*net.TCPAddr)
	if !ok {
		return bound.String()
	}
	if address.IP == nil || address.IP.IsUnspecified() {
		// An IPv6 wildcard is not necessarily reachable over IPv4: the socket
		// is only dual-stack when the system says so.
		host := "127.0.0.1"
		if address.IP != nil && address.IP.To4() == nil {
			host = "::1"
		}
		return net.JoinHostPort(host, strconv.Itoa(address.Port))
	}
	return address.String()
}

// previewHandler serves the built page with caching switched off. --serve
// rebuilds its directory on every run and the kernel hands out ephemeral ports
// that repeat, so http://127.0.0.1:PORT is not a stable identity for anything.
// Left cacheable, a reload mixes assets from an earlier run into a later page —
// visible in --split as stale colours, since that is the only mode with
// separate assets to disagree with each other.
func previewHandler(directory string) http.Handler {
	files := http.FileServer(http.Dir(directory))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		// Dropping the validators answers with the file rather than a 304, so
		// a browser still holding an entry from an earlier run on this port
		// cannot revalidate its way back to stale content.
		request.Header.Del("If-Modified-Since")
		request.Header.Del("If-None-Match")
		files.ServeHTTP(writer, request)
	})
}

func openBrowser(address string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
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

// convert writes the whole page to one writer. --split instead calls
// writePage with the data pointed at a separate file; see convertDirectory.
func convert(cli options, destination io.Writer) error {
	page := bufio.NewWriter(destination)
	if err := writePage(cli, page, page, nil); err != nil {
		return err
	}
	return page.Flush()
}

// writePage renders the page into page and the row data into data. The two are
// the same writer unless split is given, in which case the stylesheet and
// script are referenced by relative path rather than inlined.
func writePage(cli options, page, data *bufio.Writer, split *splitAssets) error {
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
	customCSS, err := readAsset("css", cli.CSS)
	if err != nil {
		return err
	}
	customJS, err := readAsset("js", cli.JS)
	if err != nil {
		return err
	}
	styleHTML := ""
	if strings.TrimSpace(customCSS) != "" {
		styleHTML = "<style>" + inlineStyle(customCSS) + "</style>\n"
	}

	themeAttribute := ""
	if cli.Theme != "" && cli.Theme != "auto" {
		themeAttribute = ` data-theme="` + html.EscapeString(cli.Theme) + `"`
	}
	pickable := themes
	if !slices.Contains(themes, cli.Theme) && cli.Theme != "" {
		pickable = append(append([]string{}, themes...), cli.Theme)
	}
	themePicker := &strings.Builder{}
	themePicker.WriteString(`<select class="csvtotable-theme" id="csvtotable-theme" aria-label="Colour theme">`)
	for _, name := range pickable {
		selected := ""
		if name == cli.Theme || (cli.Theme == "" && name == "auto") {
			selected = " selected"
		}
		label := name
		if runes := []rune(name); len(runes) > 0 {
			label = strings.ToUpper(string(runes[0])) + string(runes[1:])
		}
		fmt.Fprintf(themePicker, `<option value="%s"%s>%s</option>`, html.EscapeString(name), selected, html.EscapeString(label))
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
	stylesheetHTML := "<style>" + inlineStyle(tableCSS) + "</style>\n"
	if split != nil {
		stylesheetHTML = `<link rel="stylesheet" href="` + split.css + "\">\n"
	}
	headers := []string{}
	expectedColumns := -1
	started := false
	needsComma := false
	startOutput := func() error {
		headersJSON, err := json.Marshal(headers)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(page, "<!doctype html>\n<html lang=\"en\"%s>\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<title>%s</title>\n%s%s</head>\n<body>\n<main class=\"csvtotable\">\n%s%s\n<table class=\"csvtotable-table\" id=\"csvtotable-table\"%s></table>\n</main>\n", themeAttribute, html.EscapeString(title), stylesheetHTML, styleHTML, headerHTML, themePicker.String(), tableLabel); err != nil {
			return err
		}
		if split != nil {
			_, err = fmt.Fprintf(data, "window.csvtotableData={\"headers\":%s,\"rows\":[", headersJSON)
		} else {
			_, err = fmt.Fprintf(page, "<script id=\"csvtotable-data\" type=\"application/json\">{\"headers\":%s,\"rows\":[", headersJSON)
		}
		started = err == nil
		return err
	}
	writeRow := func(row []string) error {
		encoded, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if needsComma {
			if err := data.WriteByte(','); err != nil {
				return err
			}
		}
		if _, err := data.Write(encoded); err != nil {
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

	if split != nil {
		if _, err := io.WriteString(data, "]};\n"); err != nil {
			return err
		}
		// The data file is complete, so its hash — and therefore its name — is
		// settled and the page can link to it.
		if err := data.Flush(); err != nil {
			return err
		}
	} else if _, err := io.WriteString(page, "]}</script>\n"); err != nil {
		return err
	}
	scriptsHTML, err := scriptsMarkup(cli.Compress, customJS, split)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(page, "<script id=\"csvtotable-options\" type=\"application/json\">%s</script>\n%s</body>\n</html>\n", optionsJSON, scriptsHTML)
	return err
}

// bootstrapFormat builds the table once the frontend bundle is in scope. Every
// path ends by running it; they differ only in where the rows come from.
const bootstrapFormat = `CsvToTable.setupTheme("#csvtotable-theme");CsvToTable.table=CsvToTable.createCsvTable("#csvtotable-table",%s,JSON.parse(document.getElementById("csvtotable-options").textContent));`

var (
	bootstrap      = fmt.Sprintf(bootstrapFormat, `JSON.parse(document.getElementById("csvtotable-data").textContent)`)
	splitBootstrap = fmt.Sprintf(bootstrapFormat, "window.csvtotableData")
)

// indexFile is the only fixed name --split writes: everything it links to
// carries a hash of its contents, so a regenerated directory can never serve a
// stale asset against a fresh page. The entry point has to stay put for the
// URL to keep working, which leaves its freshness to the server, as with any
// static site.
const indexFile = "index.html"

// splitAssets are the files a --split page links to. The data file is named
// last: its hash is only known once every row has been written.
type splitAssets struct {
	css      string
	js       string
	dataName func() string
}

// hashedName inserts a content digest before the extension, so unchanged
// assets keep their URL across runs and changed ones cannot be mistaken for
// them.
func hashedName(stem, extension, content string) string {
	digest := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%s.%s%s", stem, hex.EncodeToString(digest[:])[:12], extension)
}

// inflater unpacks the gzipped bundle, builds the table, then runs whatever
// --js supplied. Appending <script> elements rather than eval keeps everything
// in global scope, which is where a top-level script would have run anyway.
const inflater = `(async()=>{
const run=(source)=>{const element=document.createElement("script");element.textContent=source;document.body.appendChild(element);};
const packed=document.getElementById("csvtotable-bundle").textContent;
const stream=new Blob([Uint8Array.from(atob(packed),(c)=>c.charCodeAt(0))]).stream();
run(await new Response(stream.pipeThrough(new DecompressionStream("gzip"))).text());
%s
const custom=document.getElementById("csvtotable-custom");
if(custom)run(custom.textContent);
})().catch((error)=>{
document.getElementById("csvtotable-table").insertAdjacentHTML("beforebegin",'<p class="csvtotable-error">This table could not be unpacked. It needs a browser with gzip DecompressionStream \u2014 Chrome 103+, Firefox 113+, or Safari 16.4+ \u2014 or a page regenerated with --no-compress.</p>');
throw error;
});`

// scriptsMarkup emits the frontend bundle, the code that builds the table, and
// any --js script. Compressed, the bundle is roughly two-fifths of its source
// size, at the cost of needing a browser that can inflate it.
//
// The custom script is inert markup in the compressed page and run by the
// inflater, because a plain <script> would execute while the bundle was still
// unpacking and find no CsvToTable.table to work with.
func scriptsMarkup(compress bool, customJS string, split *splitAssets) (string, error) {
	custom := ""
	if strings.TrimSpace(customJS) != "" {
		custom = inlineScript(customJS)
	}
	// A split page has nothing to unpack: the browser caches the assets it
	// already fetched, which is what compressing them was standing in for.
	if split != nil {
		markup := `<script src="` + split.dataName() + `"></script>` + "\n" +
			`<script src="` + split.js + `"></script>` + "\n" +
			"<script>" + splitBootstrap + "</script>\n"
		if custom != "" {
			markup += "<script>" + custom + "</script>\n"
		}
		return markup, nil
	}
	if !compress {
		markup := "<script>" + inlineScript(tableJS) + "</script>\n<script>" + bootstrap + "</script>\n"
		if custom != "" {
			markup += "<script>" + custom + "</script>\n"
		}
		return markup, nil
	}
	packed, err := packAsset(tableJS)
	if err != nil {
		return "", err
	}
	markup := `<script id="csvtotable-bundle" type="application/gzip">` + packed + "</script>\n"
	if custom != "" {
		markup += `<script id="csvtotable-custom" type="text/plain">` + custom + "</script>\n"
	}
	return markup + "<script>" + fmt.Sprintf(inflater, bootstrap) + "</script>\n", nil
}

// packAsset gzips and base64-encodes an asset for the inflater. Base64 has no
// HTML-significant characters, so the result needs none of the escaping raw
// source does.
func packAsset(source string) (string, error) {
	var packed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&packed, gzip.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := io.WriteString(writer, source); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(packed.Bytes()), nil
}

// convertDirectory writes index.html and its assets into dir as separate
// files, referenced by relative path. Opening index.html from disk still works
// — <link> and <script src> resolve over file://, unlike fetch — but the point
// is a served copy, where the browser caches the assets across pages.
func convertDirectory(cli options, dir string) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}

	// Everything is staged under a temporary name and renamed in only once the
	// conversion has succeeded. Writing in place would truncate a directory
	// that is already being served the moment anything downstream fails, and
	// would clobber an input file that happens to live in the target.
	// staged is every temporary path, tracked separately from the names they
	// will be published under: a file has to be cleaned up from the moment it
	// exists, which is before its final name is always known.
	staged := []string{}
	pending := map[string]string{}
	published := false
	defer func() {
		if published {
			return
		}
		for _, temporary := range staged {
			os.Remove(temporary)
		}
	}()
	stage := func() (*os.File, error) {
		file, err := os.CreateTemp(dir, ".csvtotable-*")
		if err != nil {
			return nil, err
		}
		staged = append(staged, file.Name())
		if err := file.Chmod(0o644); err != nil {
			file.Close()
			return nil, err
		}
		return file, nil
	}

	assets := splitAssets{
		css: hashedName("csvtotable", ".css", tableCSS),
		js:  hashedName("csvtotable", ".js", tableJS),
	}
	for name, content := range map[string]string{assets.css: tableCSS, assets.js: tableJS} {
		file, err := stage()
		if err != nil {
			return err
		}
		_, writeErr := io.WriteString(file, content)
		pending[file.Name()] = name
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return err
		}
	}

	index, err := stage()
	if err != nil {
		return err
	}
	defer index.Close()
	pending[index.Name()] = indexFile
	rows, err := stage()
	if err != nil {
		return err
	}
	defer rows.Close()

	digest := sha256.New()
	page := bufio.NewWriter(index)
	data := bufio.NewWriter(io.MultiWriter(rows, digest))
	assets.dataName = func() string {
		return "data." + hex.EncodeToString(digest.Sum(nil))[:12] + ".js"
	}

	// writePage flushes data before it asks for the name, so the digest covers
	// every row by the time the page links to it.
	if err := writePage(cli, page, data, &assets); err != nil {
		return err
	}
	if err := page.Flush(); err != nil {
		return err
	}
	pending[rows.Name()] = assets.dataName()
	if err := errors.Join(index.Close(), rows.Close()); err != nil {
		return err
	}

	for temporary, name := range pending {
		if err := os.Rename(temporary, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	published = true
	return nil
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
	return strings.TrimPrefix(string(content), "\ufeff"), nil
}

// readAsset loads an asset file named by a flag. The value is a path; a leading
// @ is accepted for symmetry with --description but carries no meaning, since
// these flags never take literal text.
func readAsset(flag, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	content, err := readText("@" + strings.TrimPrefix(path, "@"))
	if err != nil {
		return "", fmt.Errorf("%s: %w", flag, err)
	}
	return content, nil
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

// The HTML tokenizer matches end tags case-insensitively, so a plain
// strings.ReplaceAll of "</script" leaves "</SCRIPT>" free to close the
// element. Case is preserved in the replacement so string literals keep their
// value.
var (
	scriptEndTag = regexp.MustCompile(`(?i)</script`)
	styleEndTag  = regexp.MustCompile(`(?i)</style`)
)

func escapeEndTag(pattern *regexp.Regexp, source string) string {
	return pattern.ReplaceAllStringFunc(source, func(match string) string {
		return "<\\" + match[1:]
	})
}

// inlineScript makes arbitrary JavaScript safe to sit inside a <script>
// element. Besides the end tag, "<!--" puts the tokenizer into an escaped
// state in which a later "<script" switches it to double-escaped, where
// "</script>" no longer closes the element and the rest of the document is
// swallowed. Escaping "<!--" costs only the Annex B HTML comment syntax.
func inlineScript(source string) string {
	return strings.ReplaceAll(escapeEndTag(scriptEndTag, source), "<!--", `<\!--`)
}

func inlineStyle(source string) string {
	return escapeEndTag(styleEndTag, source)
}

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
