import os
import click

from csvtotable import convert


# Prompt for file overwrite
def prompt_overwrite(file_name):
    # Skip if file doesn't exist
    if not os.path.exists(file_name):
        return True

    # Prompt for file overwrite if outfile already exists
    fmt = "File ({}) already exists. Do you want to overwrite? (y/n): "
    message = fmt.format(file_name)

    click.secho(message, nl=False, fg="red")
    choice = click.getchar()
    click.echo()

    if choice not in ("y", "Y"):
        return False

    return True


@click.command()
@click.argument("input_file", type=click.Path(exists=True))
@click.argument("output_file", type=click.Path(), required=False)
@click.option("-c", "--caption", type=str, help="Table caption")
@click.option("-d", "--delimiter", type=str, default=",", help="CSV delimiter")
@click.option("-q", "--quotechar", type=str, default='"',
              help="String used to quote fields containing special characters")
@click.option("-dl", "--display-length", type=int, default=-1,
              help=("Number of rows to show by default. "
                    "Defaults to -1 (show all rows)"))
@click.option("-o", "--overwrite", default=False, is_flag=True,
              help="Overwrite the output file if exisits.")
@click.option("-s", "--serve", default=False, is_flag=True,
              help="Open output html in browser instead of writing to file.")
@click.option("-h", "--height", type=str, help="Table height in px or in %.")
@click.option("-p", "--pagination", default=True, is_flag=True,
              help="Enable/disable table pagination.")
@click.option("-vs", "--virtual-scroll", type=int, default=1000,
              help=("Number of rows after which virtual scroll is enabled."
                    "Set it to -1 to disable and 0 to always enable."))
@click.option("-nh", "--no-header", default=False, is_flag=True,
              help="Disable displaying first row as headers.")
@click.option("-e", "--export", default=True, is_flag=True,
              help="Enable filtered rows export options.")
@click.option("-eo", "--export-options", type=click.Choice(["copy", "csv", "json", "print"]),
              multiple=True, help=("Enable specific export options. By default shows all. "
              "For multiple options use -eo flag multiple times. For ex. -eo json -eo csv"))
@click.option("-ps", "--preserve-sort", default=False, is_flag=True,
              help=("Preserve the default sorting order "
              "(not using this flag will cause table to be sorted by first column)."))
@click.option("-ch", "--chart", "chart_type", type=click.Choice(["bar", "line", "pie"]),
              multiple=True, help="Add chart visualization. Can be used multiple times for multiple charts.")
@click.option("-cx", "--chart-x", type=str, multiple=True,
              help="X-axis column name for bar/line chart (use with --chart bar or --chart line)")
@click.option("-cy", "--chart-y", type=str, multiple=True,
              help="Y-axis column name(s) for bar/line chart, comma-separated for multiple series")
@click.option("-cl", "--chart-labels", type=str, multiple=True,
              help="Labels column name for pie chart (use with --chart pie)")
@click.option("-cv", "--chart-values", type=str, multiple=True,
              help="Values column name for pie chart (use with --chart pie)")
@click.option("-ct", "--chart-title", type=str, multiple=True,
              help="Title for the chart")
@click.option("-ac", "--auto-charts", default=False, is_flag=True,
              help="Automatically generate charts from data based on column types")
def cli(*args, **kwargs):
    """
    CSVtoTable commandline utility.
    """
    # Convert CSV file
    content = convert.convert(kwargs["input_file"], **kwargs)

    # Serve the temporary file in browser.
    if kwargs["serve"]:
        convert.serve(content)
    # Write to output file
    elif kwargs["output_file"]:
        # Check if file can be overwrite
        if (not kwargs["overwrite"] and
                not prompt_overwrite(kwargs["output_file"])):
            raise click.Abort()

        convert.save(kwargs["output_file"], content)
        click.secho("File converted successfully: {}".format(
            kwargs["output_file"]), fg="green")
    else:
        # If its not server and output file is missing then raise error
        raise click.BadOptionUsage("Missing argument \"output_file\".")
