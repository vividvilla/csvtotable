CSVtoTable
==========

.. image:: https://api.codacy.com/project/badge/Grade/b31a7e5d6bba4b5d9331ba05b04a12ab
   :alt: Codacy Badge
   :target: https://www.codacy.com/app/vividvilla/csvtotable?utm_source=github.com&utm_medium=referral&utm_content=vividvilla/csvtotable&utm_campaign=badger

Simple command-line utility to convert CSV files to searchable and
sortable HTML table. Supports large datasets and horizontal scrolling for large number of columns.

Demo
----

`Here is a demo`_ of `sample csv`_ file converted to HTML table.

.. image:: https://raw.githubusercontent.com/vividvilla/csvtotable/master/sample/table.gif

Installation
------------

::

    pip install --upgrade csvtotable


Get started
-----------

::

    csvtotable --help

Convert ``data.csv`` file to ``data.html`` file

::

    csvtotable data.csv data.html

Open output file in a web browser instead of writing to a file

::

    csvtotable data.csv --serve

Options
-------

::

    -c,  --caption          Table caption
    -d,  --delimiter        CSV delimiter. Defaults to ','
    -q,  --quotechar        Quote chracter. Defaults to '"'
    -dl, --display-length   Number of rows to show by default. Defaults to -1 (show all rows)
    -o,  --overwrite        Overwrite the output file if exists. Defaults to false.
    -s,  --serve            Open html output in a web browser.
    -h,  --height           Table height in px or in %. Default is 75% of the page.
    -p,  --pagination       Enable/disable pagination. Enabled by default.
    -vs, --virtual-scroll   Number of rows after which virtual scroll is enabled. Default is set to 1000 rows.
                            Set it to -1 to disable and 0 to always enable.
    -nh, --no-header        Show default headers instead of picking first row as header. Disabled by default.
    -e,  --export           Enable filtered rows export options.
    -eo, --export-options   Enable specific export options. By default shows all.
                            For multiple options use -eo flag multiple times. For ex. -eo json -eo csv
    -ch, --chart            Add chart visualization (bar, line, or pie). Can be used multiple times.
    -cx, --chart-x          X-axis column name for bar/line chart
    -cy, --chart-y          Y-axis column name(s) for bar/line chart (comma-separated for multiple series)
    -cl, --chart-labels     Labels column name for pie chart
    -cv, --chart-values     Values column name for pie chart
    -ct, --chart-title      Title for the chart
    -ac, --auto-charts      Automatically generate charts based on data types

Chart Examples
--------------

Generate a line chart showing trends over time

::

    csvtotable sales_data.csv sales.html --chart line --chart-x "Month" --chart-y "Revenue,Expenses" --chart-title "Monthly Financial Trends"

Generate a bar chart comparing values

::

    csvtotable product_sales.csv products.html --chart bar --chart-x "Product" --chart-y "Units Sold" --chart-title "Sales by Product"

Generate a pie chart showing distribution

::

    csvtotable market_share.csv market.html --chart pie --chart-labels "Company" --chart-values "Market Share" --chart-title "Market Share Distribution"

Automatically detect and generate appropriate charts

::

    csvtotable data.csv data.html --auto-charts

Multiple charts in one HTML file

::

    csvtotable sales_data.csv report.html \
        --chart line --chart-x "Month" --chart-y "Revenue" --chart-title "Revenue Trend" \
        --chart bar --chart-x "Month" --chart-y "Profit" --chart-title "Monthly Profit"

Credits
-------
`Datatables`_

.. _Here is a demo: https://cdn.rawgit.com/vividvilla/csvtotable/2.1.0/sample/goog.html
.. _sample csv: https://github.com/vividvilla/csvtotable/blob/master/sample/goog.csv
.. _Datatables: https://datatables.net