from __future__ import unicode_literals
import re
import os
import six
import uuid
import json
import time
import logging
import tempfile
import webbrowser
from io import open

import unicodecsv as csv
from jinja2 import Environment, FileSystemLoader, select_autoescape

logging.basicConfig()
logger = logging.getLogger(__package__)

package_path = os.path.dirname(os.path.abspath(__file__))
templates_dir = os.path.join(package_path, "templates")

# Initialize Jinja 2 env
env = Environment(
    loader=FileSystemLoader(templates_dir),
    autoescape=select_autoescape(["html", "xml", "j2"])
)
template = env.get_template("template.j2")

# Regex to match src property in script tags
js_src_pattern = re.compile(r'<script.*?src=\"(.*?)\".*?<\/script>',
                            re.IGNORECASE | re.MULTILINE)
# Path to JS files inside templates
js_files_path = os.path.join(package_path, templates_dir)


def convert(input_file_name, **kwargs):
    """Convert CSV file to HTML table"""
    delimiter = kwargs["delimiter"] or ","
    quotechar = kwargs["quotechar"] or "|"

    if six.PY2:
        delimiter = delimiter.encode("utf-8")
        quotechar = quotechar.encode("utf-8")

    # Read CSV and form a header and rows list
    with open(input_file_name, "rb") as input_file:
        reader = csv.reader(input_file,
                            encoding="utf-8",
                            delimiter=delimiter,
                            quotechar=quotechar)

        csv_headers = []
        if not kwargs.get("no_header"):
            # Read header from first line
            csv_headers = next(reader)

        csv_rows = [row for row in reader if row]

        # Set default column name if header is not present
        if not csv_headers and len(csv_rows) > 0:
            end = len(csv_rows[0]) + 1
            csv_headers = ["Column {}".format(n) for n in range(1, end)]

    # Render csv to HTML
    html = render_template(csv_headers, csv_rows, **kwargs)

    # Freeze all JS files in template
    return freeze_js(html)


def save(file_name, content):
    """Save content to a file"""
    with open(file_name, "w", encoding="utf-8") as output_file:
        output_file.write(content)
        return output_file.name


def serve(content):
    """Write content to a temp file and serve it in browser"""
    temp_folder = tempfile.gettempdir()
    temp_file_name = tempfile.gettempprefix() + str(uuid.uuid4()) + ".html"
    # Generate a file path with a random name in temporary dir
    temp_file_path = os.path.join(temp_folder, temp_file_name)

    # save content to temp file
    save(temp_file_path, content)

    # Open templfile in a browser
    webbrowser.open("file://{}".format(temp_file_path))

    # Block the thread while content is served
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        # cleanup the temp file
        os.remove(temp_file_path)


def render_template(table_headers, table_items, **options):
    """
    Render Jinja2 template
    """
    caption = options.get("caption") or "Table"
    display_length = options.get("display_length") or -1
    height = options.get("height") or "70vh"
    default_length_menu = [-1, 10, 25, 50]
    pagination = options.get("pagination")
    virtual_scroll_limit = options.get("virtual_scroll")

    # Change % to vh
    height = height.replace("%", "vh")

    # Header columns
    columns = []
    for header in table_headers:
        columns.append({"title": header})

    # Data table options
    datatable_options = {
        "columns": columns,
        "data": table_items,
        "iDisplayLength": display_length,
        "sScrollX": "100%",
        "sScrollXInner": "100%"
    }

    # Enable virtual scroll for rows bigger than 1000 rows
    is_paging = pagination
    virtual_scroll = False
    scroll_y = height

    if virtual_scroll_limit != -1 and len(table_items) > virtual_scroll_limit:
        virtual_scroll = True
        display_length = -1

        fmt = ("\nVirtual scroll is enabled since number of rows exceeds {limit}."
               " You can set custom row limit by setting flag -vs, --virtual-scroll."
               " Virtual scroll can be disabled by setting the value to -1 and set it to 0 to always enable.")
        logger.warn(fmt.format(limit=virtual_scroll_limit))

        if not is_paging:
            fmt = "\nPagination can not be disabled in virtual scroll mode."
            logger.warn(fmt)

        is_paging = True

    if is_paging and not virtual_scroll:
        # Add display length to the default display length menu
        length_menu = []
        if display_length != -1:
            length_menu = sorted(default_length_menu + [display_length])
        else:
            length_menu = default_length_menu

        # Set label as "All" it display length is -1
        length_menu_label = [str("All") if i == -1 else i for i in length_menu]
        datatable_options["lengthMenu"] = [length_menu, length_menu_label]
        datatable_options["iDisplayLength"] = display_length

    if is_paging:
        datatable_options["paging"] = True
    else:
        datatable_options["paging"] = False

    if scroll_y:
        datatable_options["scrollY"] = scroll_y

    if virtual_scroll:
        datatable_options["scroller"] = True
        datatable_options["bPaginate"] = False
        datatable_options["deferRender"] = True
        datatable_options["bLengthChange"] = False

    enable_export = options["export"]
    if enable_export:
        if options["export_options"]:
            allowed = list(options["export_options"])
        else:
            allowed = ["copy", "csv", "json", "print"]

        datatable_options["dom"] = "Bfrtip"
        datatable_options["buttons"] = allowed

    datatable_options_json = json.dumps(datatable_options,
                                        separators=(",", ":"))

    return template.render(title=caption or "Table",
                           caption=caption,
                           datatable_options=datatable_options_json,
                           virtual_scroll=virtual_scroll,
                           enable_export=enable_export)


def freeze_js(html):
    """
    Freeze all JS assets to the rendered html itself.
    """
    matches = js_src_pattern.finditer(html)

    if not matches:
        return html

    # Reverse regex matches to replace match string with respective JS content
    for match in reversed(tuple(matches)):
        # JS file name
        file_name = match.group(1)
        file_path = os.path.join(js_files_path, file_name)

        with open(file_path, "r", encoding="utf-8") as f:
            file_content = f.read()
        # Replace matched string with inline JS
        fmt = '<script type="text/javascript">{}</script>'
        js_content = fmt.format(file_content)
        html = html[:match.start()] + js_content + html[match.end():]

    return html
