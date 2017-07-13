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
@click.option("-q", "--quotechar", type=str, default="|",
              help="String used to quote fields containing special characters")
@click.option("-dl", "--display-length", type=int, default=-1,
              help=("Number of rows to show by default. "
                    "Defaults to -1 (show all rows)"))
@click.option("-o", "--overwrite", type=bool, default=False, is_flag=True,
              help="Overwrite the output file if exisits.")
@click.option("-s", "--serve", type=bool, default=False, is_flag=True,
              help="Open output html in browser instead of writing to file.")
def cli(input_file, output_file, caption, delimiter, quotechar,
        display_length, overwrite, serve):
    """
    CSVtoTable commandline utility.
    """
    # Convert CSV file
    content = convert.convert(input_file, caption=caption,
                              delimiter=delimiter, quotechar=quotechar,
                              display_length=display_length)

    # Serve the temporary file in browser.
    if serve:
        convert.serve(content)
    # Write to output file
    elif output_file:
        # Check if file can be overwrite
        if not overwrite and not prompt_overwrite(output_file):
            raise click.Abort()

        convert.save(output_file, content)
        click.secho("File converted successfully: {}".format(
            output_file), fg="green")
    else:
        # If its not server and output file is missing then raise error
        raise click.BadOptionUsage("Missing argument \"output_file\".")
