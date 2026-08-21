# CSVtoTable

CSVtoTable converts CSV and Excel files into interactive HTML tables.

- Single native binary with embedded frontend assets
- Standalone HTML output that works offline
- CSV and Excel (`.xlsx`) input
- Local files, URLs, standard input, gzip archives, and multi-file input
- Mobile-responsive table layout
- Search, per-column filters with active-filter chips, sorting, and virtual scrolling
- Copy, CSV, JSON, and print exports, plus column show/hide
- Markdown or raw HTML in the page title and description
- Light and dark themes that follow the system and remember a manual choice

![CSVtoTable demo](sample/table.gif)

## Usage

```sh
# Write a standalone page
csvtotable input.csv page.html

# Combine files with the same columns
csvtotable input1.csv input2.csv page.html

# Gzip-compressed CSV files
csvtotable data.csv.gz data.html

# Read an Excel workbook
csvtotable sales.xlsx sales.html

# Fetch CSV directly from a URL
csvtotable https://raw.githubusercontent.com/vividvilla/csvtotable/master/sample/meteorite-landings-1.csv meteorites.html

# Open a temporary page in the default browser
csvtotable data.csv --serve

# Add a title and generate headers for headerless data
csvtotable data.csv data.html --title "Sales" --no-header

# Title and description accept Markdown
csvtotable data.csv data.html --title "Sales for **Q3**" --description "Pulled from \`warehouse\`. See the [runbook](https://example.com)."

# or pass a markdown file as decription
csvtotable data.csv data.html --title "Sales for **Q3**" --description @sales.md

# or use raw HTML instead of Markdown
csvtotable data.csv data.html --title-html '<span>Sales <b>Q3</b></span>'
csvtotable data.csv data.html --title-html '<span>Sales <b>Q3</b></span>' --description-html @description.html

# Paginate instead of showing every row
csvtotable data.csv data.html --page-size 50

# Read stdin and write stdout
curl -L https://example.com/data.csv | csvtotable - - > data.html
```

Inputs may be local files, `http://` or `https://` URLs, or `-` for standard
input. When combining inputs, their headers must match; rows shorter than the
header are padded. With `--no-header`, generated column names are used. The
final positional argument is the output file unless `--serve` is used.

The format is detected from the content, not the file name, so it works for
URLs and standard input too. Excel workbooks are read from their first
worksheet. Gzip-compressed input is unpacked automatically, and so is
BOM-marked UTF-8 and UTF-16 input. Use `--encoding` for other encodings, and
`--delimiter` or `--quotechar` for custom CSV formats.

`--title` and `--description` are rendered as Markdown, with the description
appearing under the heading. Raw HTML in them is dropped; to pass HTML through,
use `--title-html` or `--description-html` instead. Either description flag
accepts `@FILE` to read its content from a file, like `curl`. The browser tab
uses the plain text of the title.

Show a fixed number of rows per page with `--page-size`; the default of `-1`
shows every row and lets the table scroll.

Each column gets a filter under its header: a dropdown for columns with few
distinct values, a text box otherwise. Active filters appear as chips below the
search box, where they can be cleared one at a time or all at once. Use
`--no-column-filters` to hide the filter row. The toolbar carries an export menu
and a column show/hide menu; pick a subset of either with `--export-options`.

### Styling

The page is plain semantic HTML, and every element CSVtoTable owns carries a
`csvtotable-` class. These are the stable hooks:

| Hook | Element |
| --- | --- |
| `.csvtotable` | `<main>`, the page root |
| `.csvtotable-header` | `<header>` wrapping the title and description |
| `.csvtotable-title` | `<h1>` page heading |
| `.csvtotable-description` | description block |
| `.csvtotable-table` | the `<table>` |
| `.csvtotable-theme` | light/dark toggle |
| `.csvtotable-filters` | active-filter chip row |
| `.csvtotable-chip` | one active filter, with `-key`, `-value`, and `-remove` parts |
| `.csvtotable-clear` | the "clear all" control |

Sizes and colours resolve through custom properties on `:root` — `--ct-accent`,
`--ct-paper`, `--ct-ink`, `--ct-muted`, `--ct-rule`, the `--ct-text-*` scale, and
`--ct-sans`/`--ct-mono` — so retheming usually means redefining a few of those
rather than writing rules. Headings are styled by level (`h1` to `h6`), so a
Markdown heading in a description matches the same level elsewhere; start
description headings at `##` to keep one `<h1>` per page.

Nothing is styled through an ID, so a stylesheet loaded after the page's own
`<style>` overrides any of the above with a single class selector. Classes
beginning `dt-` come from DataTables and may change when it is upgraded.

Run `csvtotable --help` for all options or `csvtotable --version` for the version.
For compatibility with version 2, `--caption`, `--display-length`, `--pagination`,
and `--export` still work; the last two disable those features.

## Install

### uvx, pipx, or pip

Run without installing:

```sh
uvx csvtotable data.csv data.html
```

Or install it:

```sh
pipx install csvtotable
uv tool install csvtotable
pip install csvtotable  # inside a virtual environment
```

### npx or npm

```sh
npx @vividvilla/csvtotable data.csv data.html   # run without installing
npm install --global @vividvilla/csvtotable     # or install globally
```

To run a particular published version:

```sh
uvx csvtotable@X.Y.Z data.csv data.html
npx @vividvilla/csvtotable@X.Y.Z data.csv data.html
```

### Homebrew

```sh
brew install vividvilla/tap/csvtotable
```

### Standalone binary

Download an archive from [GitHub Releases](https://github.com/vividvilla/csvtotable/releases),
verify it with `SHA256SUMS`, and place `csvtotable` (`csvtotable.exe` on Windows)
on your `PATH`.

Prebuilt binaries support Linux x86-64/ARM64, macOS 12+ x86-64/Apple Silicon,
and Windows 10+ x86-64.

## Development

Building requires Go 1.25+ and Bun. Distribution packaging also requires Python
and npm. `table/dist` is generated before the Go build and embedded in the final
binary.

```sh
nix develop  # optional development environment
make build   # frontend and build/csvtotable
make test    # frontend, formatting, vet, and tests
make dist    # binary and host-platform packages under dist/
```

```sh
./build/csvtotable sample/meteorite-landings-1.csv sample/meteorite-landings-2.csv /tmp/meteorites.html
```

## License

MIT
