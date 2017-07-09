from __future__ import unicode_literals
import re
import os
import six
import pandas as pd
from IPython.display import display,HTML
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
template = env.get_template("template.html")

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

    if six.PY2:
        delimiter = delimiter.encode("utf-8")
        quotechar = quotechar.encode("utf-8")

    output_file = open(output_file_name, "w", encoding="utf-8")

    # Read CSV in a dataframe
    content = pd.read_csv(input_file_name,sep=delimiter,quotechar = quotechar)

    # Add caption to the dataframe 
    html = content.style.set_caption(caption).render()
    
    # Freeze all JS files in template
    js_freezed_html = freeze_js(html)

    # Write to output
    output_file.write(js_freezed_html)

    # Close the files
    output_file.close()



def freeze_js(html):
    """
    Freeze all JS assets to the rendered html itself.
    """
    matches = js_src_pattern.finditer(html)

    if not matches:
        return html

    # Reverse regex matches to replace match string with respective JS content
    reversed_matches = reversed([m for m in matches])
    for match in reversed_matches:
        # JS file name
        file_name = match.group(1)
        file_path = os.path.join(js_files_path, file_name)

        with open(file_path, "r", encoding="utf-8") as f:
            file_content = f.read()
            # Replace matched string with inline JS
            js_content = '<script type="text/javascript">{}</script>'.format(
                file_content)
            html = html[:match.start()] + js_content + html[match.end():]

    return html
