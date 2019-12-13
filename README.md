# Archon EAD Fetcher: `eadfetch`

`eadfetch` is a small command line utility to retrieve EAD files from a running [Archon](https://github.com/archonproject/archon/) content management system.

## Why this exists

While attempting to export EAD files via the bulk download functions in Archon, we encountered two issues:

* The generated files are named using information from a collection's title. We ran into cases where the number of exported EAD records did not reflect what was in our Archon system. This appears to be due to collections with similar titles overwriting one another when exported.
* We had issues where exporting a sizable classification (~4000 records) from Archon would consistently fail. Altering various server resources, php, and Apache settings had no impact. This did not appear to be related to a specific record. Smaller classifications and individual collections would export just fine.

Rather than making chages to Archon, creating a small utility to download EAD files made the most sense for our case. The Archon EAD Fetcher is a more generalized version of what was originially written for others to use.

## Installing

There are two options for installing:

* Install/build from source.
* [Download](https://github.com/robertaltman/eadfetch/releases) a compiled binary.

### Prerequisites

[Go 1.13+](https://golang.org/dl/) is required to build from source. [`http.NewRequestWithContext`](https://golang.org/pkg/net/http/#NewRequest) from net/http is used, which was not added until Go 1.13. `eadfetch` has no dependencies outside of the Go standard library.

### Installing from Source

```
go get github.com/robertaltman/eadfetch
```

### Compiled Binary

`eadfetch` can be installed as an executable. Download the appropriate compiled binary from the [releases](https://github.com/robertaltman/eadfetch/releases) section. Place the binary in some sensible location and call directly. Open your terminal or command prompt and navigate to the location of the binary. 

```sh
Linux/Mac
./eadfetch [flags]
```

```cmd
Windows
eadfetch.exe [flags]
```

If you intend to use this tool with some regularity, consider adding the executable to your [system path](https://www.google.com/search?q=adding+an+executable+to+your+path&oq=adding+an+executable+to+your+path).

## Getting Started/Usage

Aside from a running Archon instance, the only requirement is a CSV or JSON file containing information about the collections. For ease of use, the reference point was a straight download of the `tblCollections_Collections` table from the Archon database. In both cases, only the `ID`, `CollectionIdentifier`, and `Title` fields are relevant. Strictly speaking, __only the `ID` field is required__. Removing the unused fields from either file is not necessary.

If using a CSV file, make sure the columns are named accordingly. Order does not matter, but case does.

| ID | CollectionIdentifier | Title |
|:---|:---|:---|
| 1 | RG001-01 | Some Title|
| 2 | VC123H57  | Another Example  |
| ... |   |   |

Similarly, if working with a JSON file:

```json
[
    {
        "ID" : 1,
        "CollectionIdentifier" : "VF00001",
        "Title" : "Three Dimensional (3-D) Treatment Planning,
        ...
    }
    ...
]
```

Though untested, if in a real bind with no access to a download of the database table, you could theoretically create a CSV or JSON file containing only an `ID` field/column filled in with a sequence of numbers up to some arbirary limit. If a particular `ID` does not exist in the database, the tool will take note for an output report and move on.

## How to Use

Calling `eadfetch -h` will list the configurable options. Except for the `-host` setting, which defaults to `http://127.0.0.1`, most of the options should not require much attention.

```bash
#!/bin/bash
eadfetch -h

Usage of eadfetch:
  -eadname string
        Formatting options for naming the downloaded EAD files.
        Select from one or both in any order, separated by commas: CollectionIdentifier, Title.
        (e.g., -eadname Title,CollectionIdentifier).
        Default will result in a file name of 'ead_<ID column value>.xml'.
        The ID is always appendended to the filename to ensure they are distinct. Unsafe characters will be removed.
  -file string
        File to parse for Archon data. (default "collections-table.csv")
  -host string
        Hostname of Archon site (e.g., https://archon-site.edu). (default "http://127.0.0.1")
  -output string
        Output directory for fetched XML. (default "./ead_output")
  -ratelimit int
        Limit for the number of requests to be made per second. (default 4)
  -test int
        Specifiy a number for testing a limited number of collections from the input file.
  -timeout int
        Number of seconds to allow a request to take. (default 30)
  -workers int
        Number of request workers to initiate. (default 2)
```

Below is a more friendly description of each flag:

`-eadname`

Lets you use the information in the `CollectionIdentifier` and/or `Title` fields to name the retreived EAD files. Potentially unsafe characters will be replaced with underscores. In all cases the integer from the `ID` field will be appended to the name, preceeded by an underscore. If you do not need custom names, just let it run in default mode. This will result in files named `ead_1.xml, ead_2.xml, ...`

`-file`

The relative or full path to the CSV or JSON file containing the Archon collection information. If nothing is specified, `eadfetch` will look for a file named `collections-table.csv` in the directory where the command has been called.

`-host`

The full address to your Archon server. Do not include a trailing slash. This defaults to `http://127.0.0.1`; unless you are working from an installation on your local machine, you will want to set this.

`-output`

This is the directory where you would like the files to be downloaded. Can be a relative or full path. If the folder does not exist, `eadfetch` will attempt to create it. Defaults to a folder named `ead_output` relative to where the command has been called.

`-ratelimit`

Allows the requests to be throttled and not potentially overwhelm the server. A default rate of 4 requests per second has performed well. This limit is monitored by and shared across all workers.

`-workers`

Allows for multiple request workers to be instantiated. This is helpful as one worker dealing with a slow request will not block all work from proceeding.

`-timeout`

Sets a limit on the duration of time a single request is allowed to take. If the request exceeds the number of seconds indicated here, it will be cancelled so that the worker can continue with other requests. When a request is cancelled, it will be added back into the request queue to be tried again later.

`-test`

The test flag will allow you to run `eadfetch`, but limit the number of records it attempts to process. This is helpful as you do not need to create separate files for testing; just use the full CSV/JSON and run a small test batch.

### An Example Command and Output

```bash
#!/bin/bash
eadfetch -file my_collections.csv -host http://archon-site.edu -output my_ead_files -test 40

Fetching: 40 of 40 collections.
EAD XML fetching completed in 10.093588918s.
Writing report...
The report is now complete. Look for the file 'ead-fetch-report.csv' in the my_ead_files directory.
The last column will contain information about the status of a retrieval.
```

Files should now exist in the output directory. The directory will also contain a report/log file titled `ead-fetch-report.csv` indicating the status of each request. Refer to it to determine any download issues.

## License

This project is licensed under the MIT License - see the [LICENSE](https://github.com/robertaltman/eadfetch/blob/master/LICENSE) file for details
