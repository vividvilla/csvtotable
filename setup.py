#!/usr/bin/env python

from setuptools import setup

_version = "2.1.1"

setup(
    name="csvtotable",
    version=_version,
    description="Simple commandline utility to convert CSV files"
    "to searchable and sortable HTML table.",
    author="Vivek R",
    author_email="vividvilla@gmail.com",
    url="https://github.com/vividvilla/csvtotable",
    packages=["csvtotable"],
    include_package_data=True,
    download_url="https://github.com/vividvilla/csvtotable/archive/{}.tar.gz"
        .format(_version),
    license="MIT",
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "Programming Language :: Python",
        "Natural Language :: English",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 2.6",
        "Programming Language :: Python :: 2.7",
        "Programming Language :: Python :: 3.3",
        "Programming Language :: Python :: 3.4",
        "Programming Language :: Python :: 3.5",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: Software Development :: Libraries"
    ],
    install_requires=["click >= 6.7", "jinja2 >= 2.9.6",
                      "unicodecsv >= 0.14.1", "six >= 1.10.0"],
    entry_points={
        "console_scripts": [
            "csvtotable = csvtotable.cli:cli",
            ]
    }
)
