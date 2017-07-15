Excel2Table
============

.. image:: https://api.travis-ci.org/pyexcel/excel2table.svg?branch=master
   :target: http://travis-ci.org/pyexcel/excel2table

.. image:: https://codecov.io/github/pyexcel/excel2table/coverage.png
   :target: https://codecov.io/github/pyexcel/excel2table


Simple command-line utility to convert csv, xls, xlsx, ods files to searchable and
sortable HTML table. Supports large datasets and horizontal scrolling for large number of columns.

It is a variant of `csvtotable <https://github.com/vividvilla/csvtotable>`_.

Demo
----

`Here is a demo`_ of `sample ods`_ file converted to HTML table.

.. image:: https://raw.githubusercontent.com/vividvilla/csvtotable/master/sample/table.gif

Installation
------------

::

    pip install --upgrade excel2table


Get started
-----------

::

    excel2table --help

Convert ``data.ods`` file to ``data.html`` file

::

    excel2table data.ods data.html

Open output file in a web browser instead of writing to a file

::

    excel2table data.ods --serve

Options
-------

::

    -c,  --caption          Table caption
    -d,  --delimiter        CSV delimiter. Defaults to ','
	-e,  --encoding         CSV encoding. Defaults to 'utf-8'.
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


.. _Here is a demo: https://github.com/pyexcel/excel2table/master/sample/goog.html
.. _sample ods: https://github.com/pyexcel/excel2table/blob/master/sample/goog.ods
.. _Datatables: https://datatables.net

