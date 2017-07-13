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

Credits
-------
`Datatables`_

.. _Here is a demo: https://cdn.rawgit.com/vividvilla/csvtotable/2.0.0/sample/goog.html
.. _sample csv: https://github.com/vividvilla/csvtotable/blob/master/sample/goog.csv
.. _Datatables: https://datatables.net