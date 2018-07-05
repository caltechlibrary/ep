//
// doi2eprintsxml.go is a command line utility to query CrossRef.org
// for metadata and return the results as an EPrints XML file suitable
// for importing into EPrints.
//
// Author R. S. Doiel, <rsdoiel@library.caltech.edu>
//
// Copyright (c) 2018, Caltech
// All rights not granted herein are expressly reserved by Caltech.
//
// Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//
package main

import (
	"fmt"
	"os"
	"path"

	// Caltech Library packages
	"github.com/caltechlibrary/cli"
	"github.com/caltechlibrary/crossrefapi"
	"github.com/caltechlibrary/eprinttools"
)

var (
	description = `
%s takes a one or more DOI, queries the CrossRef API
and returns an EPrints XML document suitable for import into
EPrints. The DOI can be in either their canonical form or
URL form (e.g. "10.1021/acsami.7b15651" or 
"https://doi.org/10.1021/acsami.7b15651").

`

	example = `
Example generating an EPrintsXML for one DOI

	%s "10.1021/acsami.7b15651" > article.xml

Example generating an EPrintsXML for two DOI

	%s "10.1021/acsami.7b15651" "10.1093/mnras/stu2495" > articles.xml

Example processing a list of DOIs in a text file.

	%s -i doi-list.txt -o articles.xml
`

	license = `
%s %s

Copyright (c) 2017, Caltech
All rights not granted herein are expressly reserved by Caltech.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
`

	// Standard Options
	showHelp             bool
	showLicense          bool
	showVersion          bool
	generateMarkdownDocs bool

	// App specific options
	apiEPrintsURL string
	mailto        string
)

func main() {
	appName := path.Base(os.Args[0])

	app := cli.NewCli(eprinttools.Version)
	app.AddParams("DOI")

	app.AddHelp("license",
		[]byte(fmt.Sprintf(eprinttools.LicenseText,
			appName, eprinttools.Version)))
	app.AddHelp("description", []byte(fmt.Sprintf(description, appName)))

	// Standard Options
	app.BoolVar(&showHelp, "h,help", false, "display help")
	app.BoolVar(&showLicense, "l,license", false, "display license")
	app.BoolVar(&showVersion, "v,version", false, "display app version")
	app.BoolVar(&generateMarkdownDocs, "generate-markdown-docs", false, "output documentation in Markdown")

	// Application Options
	app.StringVar(&apiEPrintsURL, "eprints-url", "", "Sets the EPRints API URL")

	app.StringVar(&mailto, "m,mailto", "", "set the mailto value for CrossRef API access")

	app.Parse()
	args := app.Args()

	if generateMarkdownDocs {
		app.GenerateMarkdownDocs(os.Stdout)
		os.Exit(0)
	}

	if showHelp {
		if showHelp {
			if len(args) > 0 {
				fmt.Fprintf(os.Stdout, app.Help(args...))
			} else {
				app.Usage(os.Stdout)
			}
			os.Exit(0)
		}
	}

	if showLicense {
		fmt.Fprintln(os.Stdout, app.License())
		os.Exit(0)
	}

	if showVersion {
		fmt.Fprintln(os.Stdout, app.Version())
		os.Exit(0)
	}

	if len(args) < 1 {
		app.Usage(os.Stderr)
		os.Exit(1)
	}

	eprintsList := new(*EPrints)
	//NOTE: need to support processing one or more DOI
	for _, doi := range args {
		api, err := crossrefapi.NewCrossRefClient(mailto)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		obj, err := api.Works(doi)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		eprintsList.EPrint = append(eprintsList.Eprint, doi2EPrint(obj))
	}
	src, err := xml.Marshal(erpintsList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "%s\n", src)
}
