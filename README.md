# CSVtoTable

CSVtoTable converts CSV files into standalone, searchable and sortable HTML tables.
The converter is a native Go executable and the generated page uses DataTables 3.

## Install the converter

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

The wheel installs a prebuilt executable; the application itself does not require
Python at runtime. `uv tool install csvtotable` works too.

### npx or npm

Run without a permanent installation:

```sh
npx @vividvilla/csvtotable data.csv data.html
```

Or install the same native executable globally:

```sh
npm install --global @vividvilla/csvtotable
```

### Manual installation

Download the archive for your system from
[GitHub Releases](https://github.com/vividvilla/csvtotable/releases), extract the
single `csvtotable` executable (`csvtotable.exe` on Windows), and put it on your
`PATH`. Releases include:

- Linux x86-64 and ARM64
- macOS 12+ x86-64 and Apple Silicon
- Windows 10+ x86-64

Use the accompanying `SHA256SUMS` file to verify the download.

Every installation method uses the same one-file native executable for its target.
The table JavaScript and CSS are compiled into that executable; no sidecar assets
are required.

## Usage

```sh
csvtotable data.csv data.html
csvtotable data.csv --serve
csvtotable --help
```

The version 2 command line remains supported:

```text
-c,  --caption, --title Table caption
-d,  --delimiter        CSV delimiter (default: ,)
-q,  --quotechar        CSV quote character (default: ")
-dl, --display-length   Rows per page; -1 shows all rows
-o,  --overwrite        Overwrite an existing output file
-s,  --serve            Open a temporary result in the default browser
-h,  --height           Table height in px or viewport %; defaults to available space
-p,  --pagination       Disable pagination (legacy flag behavior)
-vs, --virtual-scroll   Enable above this row count; -1 disables, 0 always enables
-nh, --no-header        Generate Column 1, Column 2, ... headers
-e,  --export           Disable export buttons (legacy flag behavior)
-eo, --export-options   copy, csv, json, or print; may be repeated
-ps, --preserve-sort    Preserve input order on initial display
```

BOM-marked UTF-8 and UTF-16 input is detected automatically. Other encodings can be
selected with `--encoding`, and `-` can be used for standard input or output:

```sh
curl -L https://example.com/data.csv | csvtotable - - > data.html
csvtotable --encoding windows-1252 data.csv data.html
```

Generated HTML contains its JavaScript, CSS, and data, so it works offline without
installation or a web server.

## Development

Building requires Go and Bun. Creating distributable packages also requires Python
and npm. The framework-free frontend in `table/` is an internal build input, not a
separately published package.

```sh
nix develop  # optional: provides all development dependencies
make build   # frontend bundle and build/csvtotable
make test    # frontend build, formatting, vet, and Go tests
make dist    # Python wheels and native executable under dist/
```

Run the source build directly with:

```sh
./build/csvtotable sample/goog.csv /tmp/goog.html
```

## Releasing

For a stable release, set the same `X.Y.Z` version in `pyproject.toml`,
`npm/package.json`, and every npm `optionalDependencies` entry, then push the tag:

```sh
git tag v3.0.0
git push origin v3.0.0
```

For a beta, PyPI and npm require different spellings. For example, set
`3.0.0b1` in `pyproject.toml`, set `3.0.0-beta.1` in `npm/package.json` and its
optional dependencies, then push `v3.0.0-beta.1`. GitHub marks it as a prerelease,
npm publishes it under the `beta` tag, and PyPI publishes it as a prerelease. Test
those packages with:

```sh
uvx csvtotable@3.0.0b1 data.csv data.html
npx @vividvilla/csvtotable@beta data.csv data.html
```

The release workflow cross-compiles all supported architectures, publishes PyPI and
npm packages, and creates a GitHub Release with standalone archives and checksums.
Before tagging, create a GitHub `release` environment and configure:

- PyPI trusted publisher: owner `vividvilla`, repository `csvtotable`, workflow
  `release.yml`, environment `release`. No PyPI token is needed.
- GitHub environment secret `NPM_TOKEN`: a granular npm token with read/write
  access to the `@vividvilla` packages and **Bypass 2FA** enabled.

## License

MIT
