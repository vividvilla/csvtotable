import os
import click

from csvtotable import convert


@click.command()
@click.argument("input_file", type=click.Path(exists=True))
@click.argument("output_file", type=click.Path())
@click.option("-c", "--caption", type=str, help="Table caption")
@click.option("-d", "--delimiter", type=str, default=",", help="CSV delimiter")
@click.option("-q", "--quotechar", type=str, default="|",
              help="String used to quote fields containing special characters")
def cli(input_file, output_file, caption, delimiter, quotechar):
    """
    CSVtoTable commandline utility.
    """
    # Prompt for file overwrite if outfile already exists
    if os.path.exists(output_file):
        fmt = "File ({}) already exists. Do you want to overwrite? (y/n): "
        message = fmt.format(output_file)
        click.secho(message, nl=False, fg="red")
        choice = click.getchar()
        click.echo()
        if choice not in ("y", "Y"):
            return True

    # Convert CSV file
    convert.convert(input_file, output_file, caption=caption,
                    delimiter=delimiter, quotechar=quotechar)

    click.secho("File converted successfully: {}".format(
        output_file), fg="green")
