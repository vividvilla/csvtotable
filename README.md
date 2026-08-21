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
- Five colour themes, pickable in the page or fixed with `--theme`
- Custom CSS and JavaScript inlined into the page with `--css` and `--js`

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

# Open in a particular theme
csvtotable data.csv data.html --theme solarized

# Add your own CSS and JavaScript
csvtotable data.csv data.html --css brand.css --js setup.js

# Read stdin and write stdout
curl -L https://example.com/data.csv | csvtotable - - > data.html

# Explore all the available options
csvtotable --help
```

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
| `.csvtotable-theme` | theme picker |
| `.csvtotable-filters` | active-filter chip row |
| `.csvtotable-chip` | one active filter, with `-key`, `-value`, and `-remove` parts |
| `.csvtotable-clear` | the "clear all" control |

A theme is nothing but a block of colour variables. Every rule in the
stylesheet reads them, so a new theme is a copy of one block with different
values — no rules to write and nothing else to touch:

```css
[data-theme="my-theme"] {
  color-scheme: dark;      /* so scrollbars and dropdowns match */
  --ct-paper: #1a1b26;     /* page background */
  --ct-ink: #c0caf5;       /* body text */
  --ct-muted: #7f88b0;     /* labels, secondary text */
  --ct-rule: #232433;      /* row separators */
  --ct-rule-strong: #343b58;
  --ct-accent: #7aa2f7;    /* focus, active sort, filter chips */
  --ct-wash: #1f2335;      /* accent tint behind chips */
  --ct-hover: #1e2030;     /* row and control hover */
}
```

Switching themes is just swapping `data-theme` on `<html>`. Type and metrics sit
outside the palettes in the `--ct-text-*` scale and `--ct-sans`/`--ct-mono`,
since they do not vary by theme. Headings are styled by level (`h1` to `h6`), so
a Markdown heading in a description matches the same level elsewhere; start
description headings at `##` to keep one `<h1>` per page.

Nothing is styled through an ID, so a stylesheet loaded after the page's own
`<style>` overrides any of the above with a single class selector. Classes
beginning `dt-` come from DataTables and may change when it is upgraded.

### Custom CSS and JavaScript

`--css` and `--js` inline a stylesheet and a script into the page, keeping the
output a single self-contained file. Both take a file path:

```sh
csvtotable data.csv data.html --css brand.css --js setup.js
```

A leading `@` is accepted too, for symmetry with `--description`, but means
nothing here: `--css @brand.css` and `--css brand.css` are the same.

Placement is what makes them useful. The stylesheet goes last in `<head>`, after
the built-in one, so a single class selector overrides anything above without
`!important`. The script goes last in `<body>`, after the table is built, and
`CsvToTable.table` holds the live [DataTables API](https://datatables.net/reference/api/)
instance:

```js
CsvToTable.table.order([2, "desc"]).draw();   // sort by the third column
CsvToTable.table.column(0).visible(false);    // hide the first column
```

The table's height is fitted on the next animation frame, so a script that
measures layout should wrap the read in `requestAnimationFrame`. Column widths
and the scroll height are set as inline styles, which a stylesheet cannot
override — use `--height` for that.

A whole new palette is a stylesheet plus a matching `--theme`. The name does not
have to be one of the built-ins as long as `--css` defines it, and the page's
theme picker will list it alongside them:

```sh
csvtotable data.csv data.html --css custom.css --theme tokyonight
```

Selecting "Auto" in the picker unpins `data-theme` and falls back to the
built-in light and dark palettes, so put anything that should survive that in
`:root` or a class rule and keep `[data-theme]` blocks for the palette itself.

Both flags are trusted input: whatever the files contain runs for anyone who
opens the page, so do not generate them from untrusted data. Content is escaped only
so it cannot break out of its `<style>` or `<script>` element; it is not
sanitised.

Run `csvtotable --help` for all options or `csvtotable --version` for the version.
For compatibility with version 2, `--caption`, `--display-length`, `--pagination`,
and `--export` still work; the last two disable those features.

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
