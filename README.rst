CSVtoTable
==========

Simple command-line utility to convert CSV files to searchable and
sortable HTML table.

Demo
----

`Here is a demo`_ of `sample csv`_ file converted to HTML table.

Installation
------------

::

    pip install csvtotable

Get started
-----------

::

    csvtotable --help

Convert ``data.csv`` file to ``data.html`` file

::

    csvtohtml data.csv data.html

Options
-------

::

    -c, --caption       Table caption
    -d, --delimiter     CSV delimiter. Defaults to ','
    -q, --quotechar     Quote chracter. Defaults to '|'

.. _Here is a demo: https://cdn.rawgit.com/vividvilla/csvtotable/master/sample/goog.html
.. _sample csv: https://github.com/vividvilla/csvtotable/blob/master/sample/goog.csv