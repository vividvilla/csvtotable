# CSVtoTable
Simple command-line utility to convert CSV files to searchable and sortable HTML table.

## Installation
	pip install csvtotable

## Get started
	csvtotable --help

Convert `data.csv` file to `data.html` file

	csvtohtml data.csv data.html

## Options

	-c, --caption		Table caption
	-d, --delimiter		CSV delimiter. Defaults to ','
	-q, --quotechar		Quote chracter. Defaults to '|'