from __future__ import unicode_literals
import re
import os
import six
import logging
from io import open
import unicodecsv as csv
from jinja2 import Environment, FileSystemLoader, select_autoescape

logger = logging.getLogger(__name__)

package_path = os.path.dirname(os.path.abspath(__file__))
templates_dir = os.path.join(package_path, "templates")

# Initialize Jinja 2 env
env = Environment(
    loader=FileSystemLoader(templates_dir),
    autoescape=select_autoescape(["html", "xml"])
)
template = env.get_template("template.j2")

# Regex to match src property in script tags
js_src_pattern = re.compile(r'<script.*?src=\"(.*?)\".*?<\/script>',
                            re.IGNORECASE | re.MULTILINE)
# Path to JS files inside templates
js_files_path = os.path.join(package_path, templates_dir)


def convert(input_file_name, output_file_name, **kwargs):
    """
    Convert CSV file to HTML table
    """
    caption = kwargs["caption"] or ""
    delimiter = kwargs["delimiter"] or ","
    quotechar = kwargs["quotechar"] or "|"
    display_length = kwargs["display_length"]

    if six.PY2:
        delimiter = delimiter.encode("utf-8")
        quotechar = quotechar.encode("utf-8")

    # Read CSV and form a header and rows list
    with open(input_file_name, "rb") as input_file:
        reader = csv.reader(input_file, encoding="utf-8", delimiter=delimiter,
                            quotechar=quotechar)
        # Read header from first line
        csv_headers = next(reader)
        csv_rows = [row for row in reader]

    # Template optional params
    options = {
        "caption": caption,
        "display_length": display_length
    }

    # Render csv to HTML
    html = render_template(csv_headers, csv_rows, **options)

    # Freeze all JS files in template
    js_freezed_html = freeze_js(html)

    # Write to output
    with open(output_file_name, "w", encoding="utf-8") as output_file:
        output_file.write(js_freezed_html)


def render_template(table_headers, table_items, **options):
    """
    Render Jinja2 template
    """
    caption = options["caption"] or "Table"
    display_length = options["display_length"] or -1
    default_length_menu = [-1, 10, 25, 50]

    # Add display length to the default display length menu
    length_menu = []
    if display_length != -1:
        length_menu = sorted(default_length_menu + [display_length])
    else:
        length_menu = default_length_menu

    # Set label as "All" it display length is -1
    length_menu_label = [str("All") if i == -1 else i for i in length_menu]

    return template.render(title=caption or "Table",
                           caption=caption,
                           table_headers=table_headers,
                           table_items=table_items,
                           length_menu=length_menu,
                           length_menu_label=length_menu_label,
                           display_length=display_length)


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
