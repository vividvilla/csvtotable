package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"<button class=\"csvtotable-theme\" id=\"csvtotable-theme\"",
		"CsvToTable.setupTheme(\"#csvtotable-theme\")",
		`"headers":["name","value"]`,
		`"pagination":false`,
		`"height":"50vh"`,
		"DataTables 3.0.2",
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
