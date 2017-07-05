#!/usr/bin/env python

from setuptools import setup

_version = "1.0.1"

setup(
    name="csvtotable",
    version=_version,
    description="Simple commandline utility to convert CSV files"
    "to searchable and sortable HTML table.",
    author="Vivek R",
    author_email="vividvilla@gmail.com",
    url="https://vivekr.net",
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
    install_requires=["click", "jinja2", "backports.csv"],
    entry_points={
        "console_scripts": [
            "csvtotable = csvtotable.cli:cli",
            ]
    }
)
