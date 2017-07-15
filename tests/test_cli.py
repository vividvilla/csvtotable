import os
from nose.tools import eq_
from excel2table.cli import cli
from click.testing import CliRunner


def test_cli():
    runner = CliRunner()

    result = runner.invoke(cli, ['sample/goog.ods', 'goog.html'])
    eq_(result.exit_code, 0)
    os.unlink('goog.html')
