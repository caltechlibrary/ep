# epgo

_epgo_ is a command line utility utilizing EPrints' REST API to produce alternative
feeds and formats. Currently it supports generating a feed of repository items based
on publication dates.

## Overview

USAGE: epgo [OPTIONS] [EPRINT_URI]

_epgo_ wraps the REST API for E-Prints 3.3 or better. It can return a list of uri,
a JSON view of the XML presentation as well as generates feeds and web pages.

_epgo_ can be configured with following environment variables

+ EPGO_API_URL (required) the URL to your E-Prints installation
+ EPGO_DBNAME   (required) the BoltDB name for exporting, site building, and content retrieval
+ EPGO_SITE_URL (optional) the website URL (might be the same as E-Prints)
+ EPGO_HTDOCS   (optional) the htdocs root for site building
+ EPGO_TEMPLATES (optional) the template directory to use for site building

If EPRINT_URI is provided then an individual EPrint is return as a JSON structure
(e.g. /rest/eprint/34.xml). Otherwise a list of EPrint paths are returned.


| Options               | Description |
|-----------------------|-----------------------------------------------------|
| -api	                | display EPrint REST API response                    |
| -export int           | export N EPrints to local database, if N is         |
|                       | negative export all EPrints                         |
| -build                | build pages and feeds from database                 |
| -feed-size int        | the number of items included in generated feeds     |
| -published-newest int | list the N newest published records                 |
| -published-oldest int | list the N oldest published records                 |
| -articles-newest int  | list the N newest articles                          |
| -articles-oldest int  | list the N oldest articles                          |
|-----------------------|-----------------------------------------------------|
| -js                   | run each JavaScript file, can be combined with -i   |
| -i                    | interactive JavaScript REPL                         |
| -p                    | pretty print JSON output                            |
|-----------------------|-----------------------------------------------------|
| -h                    |  display help info                                  |
| -l                    |  show license information                           |
| -v                    |  display version info                               |
|-----------------------|-----------------------------------------------------|

## JavaScript REPL

_epgo_ provides a JavaScript REPL for interactive accession to the REST API.
The repl includes support for generating Excel Workbook files (.xlsx) and
PCDM documents. With this you can use _epgo_ as a basis for integration
with other systems such as Fedora Commons 4.
