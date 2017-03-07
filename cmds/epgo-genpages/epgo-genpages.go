//
// Package epgo is a collection of structures and functions for working with the E-Prints REST API
//
// @author R. S. Doiel, <rsdoiel@caltech.edu>
//
// Copyright (c) 2017, Caltech
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
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	// Caltech Library packages
	"github.com/caltechlibrary/cli"
	"github.com/caltechlibrary/epgo"
)

var (
	usage = `USAGE: %s [OPTIONS]`

	description = `
SYNOPSIS

%s generates JSON documents in a htdocs directory tree.

CONFIGURATION

%s can be configured through setting the following environment
variables-

EPGO_DATASET    this is the dataset and collection directory (e.g. dataset/eprints)

EPGO_HTDOCS    this is the directory where the JSON documents will be written.`

	examples = `
EXAMPLE

	%s 

Generates JSON documents in EPGO_HTDOCS from EPGO_DATASET.`

	// Standard Options
	showHelp    bool
	showVersion bool
	showLicense bool
	outputFName string

	// App Options
	htdocs         string
	datasetName    string
	templatePath   string
	apiURL         string
	siteURL        string
	repositoryPath string

	buildEPrintMirror bool
)

func check(cfg *cli.Config, key, value string) string {
	if value == "" {
		log.Fatal("Missing %s_%s", cfg.EnvPrefix, strings.ToUpper(key))
		return ""
	}
	return value
}

func init() {
	// Setup options
	flag.BoolVar(&showHelp, "h", false, "display help")
	flag.BoolVar(&showHelp, "help", false, "display help")
	flag.BoolVar(&showLicense, "l", false, "display license")
	flag.BoolVar(&showLicense, "license", false, "display license")
	flag.BoolVar(&showVersion, "v", false, "display version")
	flag.BoolVar(&showVersion, "version", false, "display version")
	flag.StringVar(&outputFName, "o", "", "output filename (log message)")
	flag.StringVar(&outputFName, "output", "", "output filename (log message)")

	// App Specific options
	flag.StringVar(&htdocs, "htdocs", "", "specify where to write the HTML files to")
	flag.StringVar(&datasetName, "dataset", "", "the dataset/collection name")
	flag.StringVar(&apiURL, "api-url", "", "the EPrints API url")
	flag.StringVar(&siteURL, "site-url", "", "the website url")
	flag.StringVar(&templatePath, "template-path", "", "specify where to read the templates from")
	flag.StringVar(&repositoryPath, "repository-path", "", "specify the repository path to use for generated content")

	flag.BoolVar(&buildEPrintMirror, "build-eprint-mirror", true, "Build a mirror of EPrint content rendered as JSON documents")
}

func main() {
	appName := path.Base(os.Args[0])
	flag.Parse()

	cfg := cli.New(appName, "EPGO", fmt.Sprintf(epgo.LicenseText, appName, epgo.Version), epgo.Version)
	cfg.UsageText = fmt.Sprintf(usage, appName)
	cfg.DescriptionText = fmt.Sprintf(description, appName, appName)
	cfg.ExampleText = fmt.Sprintf(examples, appName)

	if showHelp == true {
		fmt.Println(cfg.Usage())
		os.Exit(0)
	}
	if showLicense == true {
		fmt.Println(cfg.License())
		os.Exit(0)
	}
	if showVersion == true {
		fmt.Println(cfg.Version())
		os.Exit(0)
	}

	out, err := cli.Create(outputFName, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	defer cli.CloseFile(outputFName, out)

	// Log to out
	log.SetOutput(out)

	// Check to see we can merge the required fields are merged.
	htdocs = check(cfg, "htdocs", cfg.MergeEnv("htdocs", htdocs))
	datasetName = check(cfg, "dataset", cfg.MergeEnv("dataset", datasetName))

	if htdocs != "" {
		if _, err := os.Stat(htdocs); os.IsNotExist(err) {
			os.MkdirAll(htdocs, 0775)
		}
	}

	// Create an API instance
	api, err := epgo.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	//
	// Read the dataset indicated in configuration and
	// render pages in the various formats supported.
	//
	log.Printf("%s %s\n", appName, epgo.Version)
	log.Printf("Rendering pages from %s\n", datasetName)
	err = api.BuildSite(-1, buildEPrintMirror)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Rendering complete")
}
