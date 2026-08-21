package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
)

func TestConverterCompatibility(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	output := filepath.Join(directory, "output.html")
	if err := os.WriteFile(input, []byte("name,value\nalice,\"</script><script>alert(1)</script>\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cli, err := parseArgs([]string{input, output, "-dl", "25", "-vs", "0", "-eo", "json", "-ps", "-p", "-e", "-h", "50vh", "-c", `\<Table\>`})
	if err != nil {
		t.Fatal(err)
	}
	if cli.PageSize != 25 || cli.VirtualScroll != 0 || cli.Height != "50vh" || !cli.PreserveSort || cli.Pagination || cli.ExportEnabled {
		t.Fatalf("legacy flags were not preserved: %+v", cli)
	}
	if err := run(cli); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	checks := []string{
		"<title>&lt;Table&gt;</title>",
		"<h1 class=\"csvtotable-title\" id=\"csvtotable-title\">&lt;Table&gt;</h1>",
		`aria-labelledby="csvtotable-title"`,
		"<select class=\"csvtotable-theme\" id=\"csvtotable-theme\"",
		"CsvToTable.setupTheme(\"#csvtotable-theme\")",
		`"headers":["name","value"]`,
		`"pagination":false`,
		`"height":"50vh"`,
		`<script id="csvtotable-bundle" type="application/gzip">`,
		"--ct-accent",
		`\u003c/script\u003e`,
	}
	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("generated HTML is missing %q", check)
		}
	}
	if strings.Contains(html, "<script src=") || strings.Contains(html, "<link") || strings.Contains(html, "</script><script>alert") {
		t.Fatal("generated HTML references external assets or contains unsafe script data")
	}

	alias, err := parseArgs([]string{input, filepath.Join(directory, "alias.html"), "--caption", "Alias"})
	if err != nil || alias.Title != "Alias" {
		t.Fatalf("--caption alias failed: %+v, %v", alias, err)
	}
	withoutCaption := filepath.Join(directory, "no-caption.html")
	cli, err = parseArgs([]string{input, withoutCaption})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err != nil {
		t.Fatal(err)
	}
	page, _ = os.ReadFile(withoutCaption)
	if strings.Contains(string(page), `<h1 class="csvtotable-title"`) {
		t.Fatal("empty caption rendered a heading")
	}

	for height, want := range map[string]string{"70%": `"height":"70vh"`, "calc(100% - 2rem)": `"height":"calc(100% - 2rem)"`} {
		cli.Height = height
		var rendered bytes.Buffer
		if err := convert(cli, &rendered); err != nil || !strings.Contains(rendered.String(), want) {
			t.Fatalf("height %q was rendered incorrectly: %v", height, err)
		}
	}
}

func TestCustomQuoteAndUTF16BOM(t *testing.T) {
	records, err := readAllCSV(strings.NewReader("name;value\n'alice';'one;two'\nabc'def;one\n'quoted'x;two;extra\n"), ';', '\'')
	if err != nil {
		t.Fatal(err)
	}
	if records[1][1] != "one;two" || records[2][0] != "abc'def" || records[3][0] != "quotedx" || len(records[3]) != 3 {
		t.Fatalf("custom quote parsing failed: %#v", records)
	}

	encoded := []byte{0xff, 0xfe}
	for _, value := range utf16.Encode([]rune("name,value\n測試,一\n")) {
		encoded = append(encoded, byte(value), byte(value>>8))
	}
	decoded, err := decodeInput(bytes.NewReader(encoded), "")
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(decoded)
	if err != nil || string(content) != "name,value\n測試,一\n" {
		t.Fatalf("UTF-16 decoding failed: %q, %v", content, err)
	}

	decoded, err = decodeInput(bytes.NewReader([]byte{0xef, 0xbb, 0xbf, 'n'}), "windows-1252")
	if err != nil {
		t.Fatal(err)
	}
	content, err = io.ReadAll(decoded)
	if err != nil || string(content) != "ï»¿n" {
		t.Fatalf("explicit encoding did not override BOM detection: %q, %v", content, err)
	}
}

func TestStdioArgumentsAndServeValidation(t *testing.T) {
	var help bytes.Buffer
	command := newCommand(func(options) error {
		t.Fatal("action ran without input")
		return nil
	})
	command.Writer = &help
	if err := command.Run(context.Background(), []string{"csvtotable"}); err != nil || !strings.Contains(help.String(), "USAGE:") {
		t.Fatalf("no arguments did not show help: %v\n%s", err, help.String())
	}
	cli, err := parseArgs([]string{"-", "-", "--caption", "stdin"})
	if err != nil || len(cli.InputFiles) != 1 || cli.InputFiles[0] != "-" || cli.OutputFile != "-" || cli.Title != "stdin" {
		t.Fatalf("stdio arguments were not preserved: %+v, %v", cli, err)
	}
	cli, err = parseArgs([]string{"--delimiter", "-", "input.csv", "output.html"})
	if err != nil || cli.Delimiter != "-" || len(cli.InputFiles) != 1 || cli.InputFiles[0] != "input.csv" {
		t.Fatalf("dash flag value was treated as stdin: %+v, %v", cli, err)
	}
	served, err := parseArgs([]string{"one.csv", "two.csv", "--serve"})
	if err != nil || len(served.InputFiles) != 2 || served.OutputFile != "" {
		t.Fatalf("--serve did not accept multiple inputs: %+v, %v", served, err)
	}
	merged, err := parseArgs([]string{"one.csv", "two.csv", "output.html"})
	if err != nil || len(merged.InputFiles) != 2 || merged.OutputFile != "output.html" {
		t.Fatalf("multiple positional inputs were not parsed: %+v, %v", merged, err)
	}
	if _, err := parseArgs([]string{"-", "-", "--serve"}); err == nil {
		t.Fatal("multiple stdin inputs were accepted")
	}
	directory := t.TempDir()
	output := filepath.Join(directory, "output.html")
	if err := os.WriteFile(output, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	cli, err = parseArgs([]string{"-", output})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("stdin conversion tried to read an overwrite prompt: %v", err)
	}
}

func TestMultipleInputsAndURLs(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first.csv")
	if err := os.WriteFile(first, []byte("city,temperature\nPune,29\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(output http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/missing.csv" {
			http.Error(output, "missing", http.StatusNotFound)
			return
		}
		if request.URL.Path == "/headerless.csv" {
			fmt.Fprint(output, "Mysuru,25\n")
			return
		}
		fmt.Fprint(output, "city,temperature\nMysuru,25\n")
	}))
	defer server.Close()

	cli := options{InputFiles: []string{first, server.URL + "/second.csv"}, Delimiter: ",", Quote: "\""}
	var rendered bytes.Buffer
	if err := convert(cli, &rendered); err != nil {
		t.Fatal(err)
	}
	page := rendered.String()
	for _, value := range []string{`"headers":["city","temperature"]`, `["Pune","29"]`, `["Mysuru","25"]`} {
		if !strings.Contains(page, value) {
			t.Errorf("combined output is missing %s", value)
		}
	}

	mismatchedHeader := filepath.Join(directory, "header.csv")
	if err := os.WriteFile(mismatchedHeader, []byte("city,humidity\nMumbai,70\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cli.InputFiles = []string{first, mismatchedHeader}
	if err := convert(cli, io.Discard); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched headers were accepted: %v", err)
	}
	empty := filepath.Join(directory, "empty.csv")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cli.InputFiles = []string{first, empty}
	if err := convert(cli, io.Discard); err == nil || !strings.Contains(err.Error(), "missing header") {
		t.Fatalf("input without a header was accepted: %v", err)
	}

	mismatchedColumns := filepath.Join(directory, "columns.csv")
	if err := os.WriteFile(mismatchedColumns, []byte("city,temperature\nMumbai,31,humid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cli.InputFiles = []string{first, mismatchedColumns}
	if err := convert(cli, io.Discard); err == nil || !strings.Contains(err.Error(), "3 columns; expected 2") {
		t.Fatalf("mismatched columns were accepted: %v", err)
	}

	cli.InputFiles = []string{server.URL + "/missing.csv"}
	if err := convert(cli, io.Discard); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("HTTP failure was ignored: %v", err)
	}

	headerless := filepath.Join(directory, "headerless.csv")
	if err := os.WriteFile(headerless, []byte("Pune,29\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cli = options{InputFiles: []string{headerless, server.URL + "/headerless.csv"}, Delimiter: ",", Quote: "\"", NoHeader: true}
	rendered.Reset()
	if err := convert(cli, &rendered); err != nil || !strings.Contains(rendered.String(), `"headers":["Column 1","Column 2"]`) {
		t.Fatalf("headerless inputs were not combined: %v", err)
	}
}

func TestOverwritePrompt(t *testing.T) {
	for answer, want := range map[string]bool{"\n": false, "n\n": false, "y\n": true} {
		got, err := promptOverwrite("output.html", strings.NewReader(answer))
		if err != nil || got != want {
			t.Fatalf("answer %q: got %v, %v; want %v", answer, got, err, want)
		}
	}
}

func TestOutputModeAndSymlink(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(input, []byte("name,value\nalice,1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reference := filepath.Join(directory, "reference")
	file, err := os.OpenFile(reference, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	wantMode, err := os.Stat(reference)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "output.html")
	cli, err := parseArgs([]string{input, output})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err != nil {
		t.Fatal(err)
	}
	gotMode, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if gotMode.Mode().Perm() != wantMode.Mode().Perm() {
		t.Fatalf("new output mode is %o; want umask-adjusted %o", gotMode.Mode().Perm(), wantMode.Mode().Perm())
	}

	target := filepath.Join(directory, "target.html")
	link := filepath.Join(directory, "link.html")
	if err := os.WriteFile(target, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cli, err = parseArgs([]string{"--overwrite", input, link})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err != nil {
		t.Fatal(err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil || linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("output symlink was replaced: %v", err)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if targetInfo.Mode().Perm() != 0o640 || !bytes.Contains(page, []byte("<!doctype html>")) {
		t.Fatalf("symlink target was not safely replaced: mode=%o", targetInfo.Mode().Perm())
	}

	badInput := filepath.Join(directory, "bad.csv")
	if err := os.WriteFile(badInput, []byte("name,value\nalice,\"unterminated"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	cli, err = parseArgs([]string{"--overwrite", badInput, link})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err == nil {
		t.Fatal("malformed CSV was accepted")
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("failed conversion changed the existing output")
	}
	failedOutput := filepath.Join(directory, "failed.html")
	cli, err = parseArgs([]string{badInput, failedOutput})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err == nil {
		t.Fatal("malformed CSV was accepted")
	}
	if _, err := os.Stat(failedOutput); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed conversion left a new output file")
	}
}

func TestRejectsMatchingDelimiterAndQuote(t *testing.T) {
	cli := options{InputFiles: []string{"-"}, Delimiter: ",", Quote: ",", ExportOptions: []string{}}
	if err := convert(cli, io.Discard); err == nil {
		t.Fatal("matching delimiter and quotechar were accepted")
	}
}

func readAllCSV(input *strings.Reader, delimiter, quote rune) ([][]string, error) {
	reader := newCSVReader(input, delimiter, quote)
	var records [][]string
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return records, nil
			}
			return nil, err
		}
		records = append(records, record)
	}
}

func TestGzipInputs(t *testing.T) {
	directory := t.TempDir()
	plain := "city,temperature\nPune,29\n"
	compressed := filepath.Join(directory, "input.csv.gz")
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := io.WriteString(writer, plain); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(compressed, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(output http.ResponseWriter, _ *http.Request) {
		output.Write(buffer.Bytes())
	}))
	defer server.Close()

	uncompressed := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(uncompressed, []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := convert(options{InputFiles: []string{uncompressed}, Delimiter: ",", Quote: "\""}, &want); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{compressed, server.URL + "/input.csv.gz"} {
		var rendered bytes.Buffer
		if err := convert(options{InputFiles: []string{source}, Delimiter: ",", Quote: "\""}, &rendered); err != nil {
			t.Fatalf("gzip input %q: %v", source, err)
		}
		if rendered.String() != want.String() {
			t.Errorf("gzip input %q did not match uncompressed output", source)
		}
	}

	truncated := filepath.Join(directory, "truncated.csv.gz")
	if err := os.WriteFile(truncated, buffer.Bytes()[:buffer.Len()-4], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := convert(options{InputFiles: []string{truncated}, Delimiter: ",", Quote: "\""}, io.Discard); err == nil {
		t.Error("truncated gzip input was accepted")
	}
}

func TestToolbarAndColumnFilterOptions(t *testing.T) {
	parsed, err := parseArgs([]string{"--export-options", "colvis", "input.csv", "output.html"})
	if err != nil {
		t.Fatalf("colvis was rejected: %v", err)
	}
	if !parsed.ColumnFilters {
		t.Error("column filters are not enabled by default")
	}
	if _, err := parseArgs([]string{"--export-options", "pdf", "input.csv", "output.html"}); err == nil {
		t.Error("unknown toolbar button was accepted")
	}
	parsed, err = parseArgs([]string{"--no-column-filters", "input.csv", "output.html"})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ColumnFilters {
		t.Error("--no-column-filters did not disable column filters")
	}
}

func TestTitleAndDescriptionRendering(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(input, []byte("city,temperature\nPune,29\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(directory, "notes.md")
	if err := os.WriteFile(notes, []byte("Pulled from the **warehouse**.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := options{InputFiles: []string{input}, Delimiter: ",", Quote: "\""}

	markdown := base
	markdown.Title = "Sales *for* `Q3`"
	markdown.Description = "@" + notes
	var rendered bytes.Buffer
	if err := convert(markdown, &rendered); err != nil {
		t.Fatal(err)
	}
	page := rendered.String()
	for _, want := range []string{
		`<h1 class="csvtotable-title" id="csvtotable-title">Sales <em>for</em> <code>Q3</code></h1>`,
		`<div class="csvtotable-description"><p>Pulled from the <strong>warehouse</strong>.</p></div>`,
		"<title>Sales for Q3</title>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("rendered page is missing %s", want)
		}
	}

	raw := base
	raw.TitleHTML = `<span>Ops <b>Dash</b></span>`
	raw.DescriptionHTML = `<p>Raw <a href="#">link</a></p>`
	rendered.Reset()
	if err := convert(raw, &rendered); err != nil {
		t.Fatal(err)
	}
	page = rendered.String()
	for _, want := range []string{
		`<h1 class="csvtotable-title" id="csvtotable-title"><span>Ops <b>Dash</b></span></h1>`,
		`<div class="csvtotable-description"><p>Raw <a href="#">link</a></p></div>`,
		"<title>Ops Dash</title>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("raw HTML page is missing %s", want)
		}
	}

	unsafe := base
	unsafe.Title = "Hi <script>alert(1)</script>"
	rendered.Reset()
	if err := convert(unsafe, &rendered); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), "<script>alert(1)") {
		t.Error("Markdown mode passed raw HTML through")
	}

	missing := base
	missing.Description = "@" + filepath.Join(directory, "absent.md")
	if err := convert(missing, io.Discard); err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("missing description file was accepted: %v", err)
	}

	if _, err := parseArgs([]string{"--title", "a", "--title-html", "b", input, "out.html"}); err == nil {
		t.Error("--title and --title-html were accepted together")
	}
	if _, err := parseArgs([]string{"--description", "a", "--description-html", "b", input, "out.html"}); err == nil {
		t.Error("--description and --description-html were accepted together")
	}
	sized, err := parseArgs([]string{"--page-size", "25", input, "out.html"})
	if err != nil || sized.PageSize != 25 {
		t.Fatalf("--page-size failed: %+v, %v", sized, err)
	}
}

func TestWorkbookInput(t *testing.T) {
	directory := t.TempDir()
	workbook := excelize.NewFile()
	defer workbook.Close()
	sheet := workbook.GetSheetName(0)
	for index, row := range [][]any{
		{"city", "temperature", "note"},
		{"Pune", 29, "warm"},
		{"Mumbai", 31},
	} {
		cell, err := excelize.CoordinatesToCellName(1, index+1)
		if err != nil {
			t.Fatal(err)
		}
		if err := workbook.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(directory, "input.xlsx")
	if err := workbook.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	var rendered bytes.Buffer
	if err := convert(options{InputFiles: []string{path}, Delimiter: ",", Quote: "\""}, &rendered); err != nil {
		t.Fatal(err)
	}
	page := rendered.String()
	for _, want := range []string{
		`"headers":["city","temperature","note"]`,
		`["Pune","29","warm"]`,
		`["Mumbai","31",""]`, // trailing empty cells are dropped by the format
	} {
		if !strings.Contains(page, want) {
			t.Errorf("workbook output is missing %s", want)
		}
	}

	notWorkbook := filepath.Join(directory, "fake.xlsx")
	if err := os.WriteFile(notWorkbook, []byte("PK\x03\x04not a workbook"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := convert(options{InputFiles: []string{notWorkbook}, Delimiter: ",", Quote: "\""}, io.Discard); err == nil {
		t.Error("a corrupt workbook was accepted")
	}
}

func TestThemes(t *testing.T) {
	// The CLI list and the stylesheet's [data-theme] blocks have to agree, or
	// --theme silently produces an unstyled page.
	for _, name := range themes {
		if name == "auto" {
			continue
		}
		// The minifier drops the attribute-value quotes.
		if !strings.Contains(tableCSS, "[data-theme="+name+"]") {
			t.Errorf("stylesheet has no block for theme %q", name)
		}
	}

	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(input, []byte("city,temperature\nPune,29\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := options{InputFiles: []string{input}, Delimiter: ",", Quote: "\""}

	named := base
	named.Theme = "nord"
	var rendered bytes.Buffer
	if err := convert(named, &rendered); err != nil {
		t.Fatal(err)
	}
	page := rendered.String()
	for _, want := range []string{
		`<html lang="en" data-theme="nord">`,
		`<option value="nord" selected>Nord</option>`,
		`<select class="csvtotable-theme" id="csvtotable-theme" aria-label="Colour theme">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("themed page is missing %s", want)
		}
	}

	// "auto" pins nothing, leaving the stylesheet to follow the system.
	auto := base
	auto.Theme = "auto"
	rendered.Reset()
	if err := convert(auto, &rendered); err != nil {
		t.Fatal(err)
	}
	// The stylesheet itself is full of [data-theme=...] selectors, so look at
	// the root element specifically.
	if strings.Contains(rendered.String(), `<html lang="en" data-theme=`) {
		t.Error("auto pinned a theme on the root element")
	}

	parsed, err := parseArgs([]string{"--theme", "gruvbox", input, "out.html"})
	if err != nil || parsed.Theme != "gruvbox" {
		t.Fatalf("--theme gruvbox failed: %+v, %v", parsed, err)
	}
	if parsed, err := parseArgs([]string{input, "out.html"}); err != nil || parsed.Theme != "auto" {
		t.Fatalf("theme did not default to auto: %+v, %v", parsed, err)
	}
	if _, err := parseArgs([]string{"--theme", "bogus", input, "out.html"}); err == nil {
		t.Error("an unknown theme was accepted")
	}
}

func TestCustomCSSAndJS(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(input, []byte("city,temperature\nPune,29\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := options{InputFiles: []string{input}, Delimiter: ",", Quote: "\""}

	write := func(name, content string) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	css, js := ".csvtotable-title{color:red}", "CsvToTable.table.draw()"

	cli := base
	cli.CSS = write("custom.css", css)
	cli.JS = write("custom.js", js)
	var rendered bytes.Buffer
	if err := convert(cli, &rendered); err != nil {
		t.Fatal(err)
	}
	page := rendered.String()

	// Placement is the whole feature: CSS after the built-in sheet so it wins
	// on equal specificity, JS after the bootstrap so the table exists.
	style := strings.Index(page, "<style>"+css+"</style>")
	if style < 0 {
		t.Fatal("custom CSS was not emitted")
	}
	if style < strings.Index(page, "<style>:root") || style > strings.Index(page, "</head>") {
		t.Error("custom CSS is not the last thing in <head>")
	}
	script := strings.Index(page, "<script>"+js+"</script>")
	if script < 0 {
		t.Fatal("custom JS was not emitted")
	}
	if script < strings.Index(page, "CsvToTable.table=CsvToTable.createCsvTable") {
		t.Error("custom JS runs before the table is built")
	}
	if script > strings.Index(page, "</body>") {
		t.Error("custom JS is outside <body>")
	}

	// User content must not be able to close the tag it sits in.
	breakout := base
	breakout.CSS = write("breakout.css", "</style><b>x")
	breakout.JS = write("breakout.js", "</script><b>x")
	rendered.Reset()
	if err := convert(breakout, &rendered); err != nil {
		t.Fatal(err)
	}
	page = rendered.String()
	if strings.Contains(page, "<style></style>") || strings.Contains(page, "<script></script>") {
		t.Error("custom content broke out of its tag")
	}
	for _, want := range []string{`<\/style`, `<\/script`} {
		if !strings.Contains(page, want) {
			t.Errorf("custom content is missing the %s escape", want)
		}
	}

	// A leading @ is tolerated for symmetry with --description but means
	// nothing here, so both spellings must produce the same page.
	stylesheet := write("extra.css", ".from-file{color:blue}")
	var plain, prefixed bytes.Buffer
	bare, at := base, base
	bare.CSS, at.CSS = stylesheet, "@"+stylesheet
	if err := convert(bare, &plain); err != nil {
		t.Fatal(err)
	}
	if err := convert(at, &prefixed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain.String(), ".from-file{color:blue}") {
		t.Error("stylesheet was not inlined")
	}
	if plain.String() != prefixed.String() {
		t.Error("a leading @ changed the output")
	}
	for flag, cli := range map[string]options{
		"css": {InputFiles: []string{input}, Delimiter: ",", Quote: "\"", CSS: filepath.Join(directory, "absent.css")},
		"js":  {InputFiles: []string{input}, Delimiter: ",", Quote: "\"", JS: filepath.Join(directory, "absent.js")},
	} {
		if err := convert(cli, io.Discard); err == nil || !strings.HasPrefix(err.Error(), flag+":") {
			t.Errorf("missing %s file gave %v", flag, err)
		}
	}

	// Nothing is emitted when the flags are unused.
	rendered.Reset()
	if err := convert(base, &rendered); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), "<style></style>") || strings.Contains(rendered.String(), "<script></script>") {
		t.Error("empty custom CSS or JS emitted a bare tag")
	}
}

func TestCustomTheme(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(input, []byte("city,temperature\nPune,29\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A palette the stylesheet has never heard of is fine as long as --css can
	// supply it, and the picker has to offer it or it would report the wrong
	// theme with no way back.
	stylesheet := filepath.Join(directory, "tokyonight.css")
	if err := os.WriteFile(stylesheet, []byte(`[data-theme="tokyonight"]{--ct-paper:#1a1b26}`), 0o644); err != nil {
		t.Fatal(err)
	}
	custom := options{InputFiles: []string{input}, Delimiter: ",", Quote: "\"",
		Theme: "tokyonight", CSS: stylesheet}
	var rendered bytes.Buffer
	if err := convert(custom, &rendered); err != nil {
		t.Fatal(err)
	}
	page := rendered.String()
	for _, want := range []string{
		`<html lang="en" data-theme="tokyonight">`,
		`<option value="tokyonight" selected>Tokyonight</option>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("custom-themed page is missing %s", want)
		}
	}

	if _, err := parseArgs([]string{"--theme", "tokyonight", "--css", stylesheet, input, "out.html"}); err != nil {
		t.Errorf("--theme with --css was rejected: %v", err)
	}
	if _, err := parseArgs([]string{"--theme", "tokyonight", input, "out.html"}); err == nil {
		t.Error("an unknown theme was accepted without --css")
	}
}

func TestInlineAssetEscaping(t *testing.T) {
	// The HTML tokenizer matches end tags case-insensitively, and "<!--"
	// followed by "<script" switches it to a state where "</script>" no longer
	// closes the element. Both silently swallowed the rest of the document.
	for _, source := range []string{`x="</SCRIPT>"`, `x="</ScRiPt >"`, `a="<!--"; b="<script /"`} {
		escaped := inlineScript(source)
		if scriptEndTag.MatchString(strings.ReplaceAll(escaped, `<\/`, "")) {
			t.Errorf("inlineScript left a live end tag in %q -> %q", source, escaped)
		}
		if strings.Contains(escaped, "<!--") {
			t.Errorf("inlineScript left a comment opener in %q -> %q", source, escaped)
		}
	}
	// Case is preserved so string literals keep their value.
	if got := inlineScript(`"</SCRIPT>"`); got != `"<\/SCRIPT>"` {
		t.Errorf("inlineScript changed the case of the end tag: %q", got)
	}
	if got := inlineStyle(`a{content:"</STYLE>"}`); !strings.Contains(got, `<\/STYLE>`) {
		t.Errorf("inlineStyle did not escape an uppercase end tag: %q", got)
	}

	// The embedded assets carry the same hazard, not just user content.
	for name, asset := range map[string]string{"css": tableCSS, "js": tableJS} {
		if strings.Contains(name, "js") && strings.Contains(inlineScript(asset), "<!--") {
			t.Error("embedded bundle still contains a comment opener after escaping")
		}
	}

	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(input, []byte("city,temperature\nPune,29\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A custom theme name reaches the picker markup and must be escaped.
	var rendered bytes.Buffer
	hostile := options{InputFiles: []string{input}, Delimiter: ",", Quote: "\"",
		Theme: `"><script>alert(1)</script>`, CSS: filepath.Join(directory, "theme.css")}
	if err := os.WriteFile(hostile.CSS, []byte("x{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := convert(hostile, &rendered); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), `<option value=""><script>`) {
		t.Error("a theme name broke out of the picker option")
	}

}

func TestPaletteOverrideSpecificity(t *testing.T) {
	// The auto-dark block must not out-weigh a plain :root rule, or a --css
	// palette override is silently ignored on a dark-mode machine.
	if strings.Contains(tableCSS, ":root:not([data-theme])") {
		t.Error("the auto-dark palette outweighs a :root override; wrap its :not() in :where()")
	}
	if !strings.Contains(tableCSS, ":root:where(:not([data-theme]))") {
		t.Error("the auto-dark palette no longer scopes itself to unpinned pages")
	}
}

func TestCustomThemeIsDiscoverable(t *testing.T) {
	// A --css theme is only reachable if the picker lists it. Go names the one
	// passed to --theme; the frontend finds the rest by reading the stylesheet,
	// so both halves have to agree on the attribute spelling.
	if !strings.Contains(tableJS, "data-theme=") {
		t.Error("the bundle no longer looks for [data-theme] blocks in loaded stylesheets")
	}
	for _, name := range themes {
		if name == "auto" {
			continue
		}
		if !strings.Contains(tableCSS, "[data-theme="+name+"]") {
			t.Errorf("stylesheet has no block for theme %q", name)
		}
	}
}

func TestBundleCompression(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(input, []byte("city,temperature\nPune,29\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(directory, "custom.js")
	if err := os.WriteFile(script, []byte("CsvToTable.table.draw()"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := options{InputFiles: []string{input}, Delimiter: ",", Quote: "\"", JS: script}

	var plain, packed bytes.Buffer
	if err := convert(base, &plain); err != nil {
		t.Fatal(err)
	}
	compressed := base
	compressed.Compress = true
	if err := convert(compressed, &packed); err != nil {
		t.Fatal(err)
	}

	// The bundle is the bulk of an otherwise empty page, so compressing it has
	// to show up as a substantially smaller file.
	if packed.Len() > plain.Len()*3/5 {
		t.Errorf("compressed page is %d bytes against %d uncompressed", packed.Len(), plain.Len())
	}
	if !strings.Contains(plain.String(), "DataTables 3.0.2") {
		t.Error("uncompressed page is missing the bundle source")
	}
	if strings.Contains(packed.String(), "DataTables 3.0.2") {
		t.Error("compressed page still carries the bundle source")
	}
	// The stylesheet stays readable: compressing it would leave the page
	// unstyled until the inflater ran.
	if !strings.Contains(packed.String(), "--ct-accent") {
		t.Error("compressed page should keep its stylesheet inline")
	}

	page := packed.String()
	blob := strings.Index(page, `<script id="csvtotable-bundle" type="application/gzip">`)
	custom := strings.Index(page, `<script id="csvtotable-custom" type="text/plain">`)
	run := strings.Index(page, "DecompressionStream")
	if blob < 0 || custom < 0 || run < 0 {
		t.Fatalf("compressed page is missing a script: blob=%d custom=%d inflater=%d", blob, custom, run)
	}
	// The inflater reads both blobs, so both must already be parsed. The
	// custom script is inert markup until the inflater runs it, because a live
	// <script> would execute before the table existed.
	if run < blob || run < custom {
		t.Error("the inflater runs before the payloads it reads")
	}
	if strings.Contains(page, "<script>CsvToTable.table.draw()</script>") {
		t.Error("custom JS runs on its own, before the bundle is unpacked")
	}

	// Decoding the blob has to give back the bundle byte for byte.
	encoded := page[blob+len(`<script id="csvtotable-bundle" type="application/gzip">`):]
	encoded = encoded[:strings.Index(encoded, "</script>")]
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("bundle is not valid base64: %v", err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("bundle is not gzip: %v", err)
	}
	unpacked, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(unpacked) != tableJS {
		t.Error("the unpacked bundle differs from the embedded one")
	}
}

func TestSplitOutput(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.csv")
	if err := os.WriteFile(input, []byte("city,temperature\nPune,29\nKochi,31\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "site")
	cli, err := parseArgs([]string{input, target, "--split", "--title", "Weather"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err != nil {
		t.Fatal(err)
	}

	read := func(name string) string {
		content, err := os.ReadFile(filepath.Join(target, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(content)
	}
	jsName := hashedName("csvtotable", ".js", tableJS)
	cssName := hashedName("csvtotable", ".css", tableCSS)
	if read(jsName) != tableJS || read(cssName) != tableCSS {
		t.Error("the written assets differ from the embedded ones")
	}

	// The page must reference its assets by relative path, so that it works
	// from a subdirectory of whatever ends up serving it, and every reference
	// must carry a content hash so a redeploy cannot be served stale.
	page := read(indexFile)
	for _, want := range []string{
		`<link rel="stylesheet" href="` + cssName + `">`,
		`<script src="` + jsName + `"></script>`,
		"window.csvtotableData",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("index.html is missing %q", want)
		}
	}
	dataRef := regexp.MustCompile(`<script src="(data\.[0-9a-f]{12}\.js)"></script>`).FindStringSubmatch(page)
	if dataRef == nil {
		t.Fatalf("index.html has no hashed data reference:\n%s", page)
	}
	dataName := dataRef[1]
	// Nothing that belongs in a separate file may also be inlined, or the
	// caching the mode exists for is wasted.
	for _, unwanted := range []string{"DataTables 3.0.2", "--ct-accent", "csvtotable-bundle", `"rows":[`} {
		if strings.Contains(page, unwanted) {
			t.Errorf("index.html still inlines %q", unwanted)
		}
	}
	if len(page) > 4096 {
		t.Errorf("index.html is %d bytes; it should hold no payload", len(page))
	}

	// The rows have to survive the trip into a separate script.
	data := read(dataName)
	encoded, ok := strings.CutPrefix(strings.TrimSpace(data), "window.csvtotableData=")
	if !ok {
		t.Fatalf("data.js does not assign the payload: %.60q", data)
	}
	var payload struct {
		Headers []string   `json:"headers"`
		Rows    [][]string `json:"rows"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSuffix(encoded, ";")), &payload); err != nil {
		t.Fatalf("data.js is not valid JSON: %v", err)
	}
	if len(payload.Headers) != 2 || len(payload.Rows) != 2 || payload.Rows[1][0] != "Kochi" {
		t.Errorf("unexpected payload: %+v", payload)
	}

	// A rerun overwrites its own files and must not need a prompt when told.
	cli, err = parseArgs([]string{input, target, "--split", "--overwrite"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err != nil {
		t.Fatalf("rerunning with --overwrite failed: %v", err)
	}

	// Different rows must land on a different URL, or a browser holding the
	// old data.js serves it against the new page.
	if err := os.WriteFile(input, []byte("city,temperature\nPune,29\nKochi,31\nDelhi,18\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(cli); err != nil {
		t.Fatal(err)
	}
	if again := read(indexFile); strings.Contains(again, dataName) {
		t.Errorf("changed rows kept the data URL %q", dataName)
	}

	// A conversion that fails must leave the served directory as it was.
	before := read(indexFile)
	broken := cli
	broken.CSS = filepath.Join(directory, "missing.css")
	if err := convertDirectory(broken, target); err == nil {
		t.Error("a missing --css file was accepted")
	}
	if after := read(indexFile); after != before {
		t.Error("a failed conversion left the previous index.html damaged")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".csvtotable-") {
			t.Errorf("a failed conversion left the staging file %q behind", entry.Name())
		}
	}
}

func TestPreviewIsNotCached(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, indexFile), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := previewHandler(directory)

	// --serve rebuilds its directory every run onto a port the kernel reuses,
	// so anything the browser keeps is a stale asset waiting to be mixed into
	// a later page.
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control is %q, want no-store", got)
	}
	if recorder.Body.String() != "first" {
		t.Errorf("served %q, want the file contents", recorder.Body.String())
	}

	if err := os.WriteFile(filepath.Join(directory, indexFile), []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("If-Modified-Since", time.Now().UTC().Format(http.TimeFormat))
	handler.ServeHTTP(recorder, request)
	if recorder.Body.String() != "second" {
		t.Errorf("a rewritten file served %q; the preview must not go stale", recorder.Body.String())
	}
}

func TestServeAddress(t *testing.T) {
	// --serve takes an optional value, which urfave/cli has no notion of, so a
	// bare --serve must not swallow the argument after it.
	forms := []struct {
		args    []string
		serve   bool
		address string
		inputs  int
	}{
		{args: []string{"in.csv", "--serve"}, serve: true, inputs: 1},
		{args: []string{"--serve", "in.csv"}, serve: true, inputs: 1},
		{args: []string{"-s", "in.csv"}, serve: true, inputs: 1},
		{args: []string{"--serve", ":8080", "in.csv"}, serve: true, address: ":8080", inputs: 1},
		{args: []string{"--serve=127.0.0.1:8080", "in.csv"}, serve: true, address: "127.0.0.1:8080", inputs: 1},
		{args: []string{"--serve", "[::1]:8080", "in.csv"}, serve: true, address: "[::1]:8080", inputs: 1},
		{args: []string{"in.csv", "out.html"}, serve: false, inputs: 1},
	}
	// A -s that belongs to another flag is that flag's value, not this one.
	guarded, err := parseArgs([]string{"--title", "-s", "in.csv", "out.html"})
	if err != nil {
		t.Fatal(err)
	}
	if guarded.Title != "-s" || guarded.Serve || len(guarded.InputFiles) != 1 {
		t.Errorf("--title -s was rewritten: title=%q serve=%v inputs=%v",
			guarded.Title, guarded.Serve, guarded.InputFiles)
	}
	for _, form := range forms {
		cli, err := parseArgs(form.args)
		if err != nil {
			t.Errorf("%v: %v", form.args, err)
			continue
		}
		if cli.Serve != form.serve || cli.Address != form.address || len(cli.InputFiles) != form.inputs {
			t.Errorf("%v: serve=%v address=%q inputs=%v; want %v, %q, %d",
				form.args, cli.Serve, cli.Address, cli.InputFiles, form.serve, form.address, form.inputs)
		}
	}

	// An empty host binds loopback: putting the data on the network should
	// take more than leaving the host off.
	targets := map[string]string{
		"":               "127.0.0.1:0",
		":8080":          "127.0.0.1:8080",
		"localhost:8080": "localhost:8080",
		"0.0.0.0:8080":   "0.0.0.0:8080",
		"[::1]:8080":     "[::1]:8080",
	}
	for address, want := range targets {
		got, err := listenTarget(address)
		if err != nil || got != want {
			t.Errorf("listenTarget(%q) = %q, %v; want %q", address, got, err, want)
		}
	}
	for _, bad := range []string{"8080", "nonsense", ":99999", "host:port"} {
		if _, err := listenTarget(bad); err == nil {
			t.Errorf("listenTarget(%q) was accepted", bad)
		}
	}

	for bind, want := range map[string]bool{
		"127.0.0.1:80": true, "localhost:80": true, "[::1]:80": true,
		"0.0.0.0:80": false, "10.0.0.4:80": false,
	} {
		if got := loopbackOnly(bind); got != want {
			t.Errorf("loopbackOnly(%q) = %v, want %v", bind, got, want)
		}
	}
}
