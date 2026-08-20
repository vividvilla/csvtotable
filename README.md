# CSVtoTable

CSVtoTable converts CSV files into interactive HTML tables.

- Single native binary with embedded frontend assets
- Standalone HTML output that works offline
- Mobile-responsive table layout
- Search, sorting, pagination, and virtual scrolling
- Copy, CSV, JSON, and print exports
- User-controlled light and dark themes

## Install

### uvx, pipx, or pip

Run without installing:

```sh
uvx csvtotable data.csv data.html
```

Or install it:

```sh
pipx install csvtotable
# or, inside a virtual environment:
pip install csvtotable
```

`uv tool install csvtotable` works too. Python is only needed to install the
prebuilt executable.

### npx or npm

```sh
npx @vividvilla/csvtotable data.csv data.html
# or install globally:
npm install --global @vividvilla/csvtotable
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

## Usage

```sh
# Write a standalone page
csvtotable data.csv data.html

# Open a temporary page in the default browser
csvtotable data.csv --serve

# Add a title and generate headers for headerless data
csvtotable data.csv data.html --title "Sales" --no-header

# Read stdin and write stdout
curl -L https://example.com/data.csv | csvtotable - - > data.html
```

BOM-marked UTF-8 and UTF-16 input is detected automatically. Use `--encoding`
for other encodings, and `--delimiter` or `--quotechar` for custom CSV formats.

Run `csvtotable --help` for all options or `csvtotable --version` for the version.
For compatibility with version 2, `--pagination` and `--export` disable those
features.

## Development

Building requires Go 1.24+ and Bun. Distribution packaging also requires Python
and npm. `table/dist` is generated before the Go build and embedded in the final
binary.

```sh
nix develop  # optional development environment
make build   # frontend and build/csvtotable
make test    # frontend, formatting, vet, and tests
make dist    # binary and host-platform packages under dist/
```

```sh
./build/csvtotable sample/goog.csv /tmp/goog.html
```

## License

MIT
