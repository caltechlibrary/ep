//
// Package epgo is a collection of structures and functions for working with the E-Prints REST API
//
// @author R. S. Doiel, <rsdoiel@caltech.edu>
//
// Copyright (c) 2016, Caltech
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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"text/template"

	// Caltech Library packages
	"github.com/caltechlibrary/cli"
	"github.com/caltechlibrary/epgo"
	"github.com/caltechlibrary/tmplfn"

	// 3rd Party packages
	"github.com/blevesearch/bleve"
)

var (
	usage = `USAGE: %s [OPTIONS]`

	description = `

 OVERVIEW

	%s a webserver for explosing EPrints as HTML pages,  HTML .include pages, JSON and BibTeX formats.

 CONFIGURATION

    %s can be configured through setting the following environment
	variables-

    EPGO_BLEVE    a colon delimited list of Bleve indexes

    EPGO_HTDOCS    this is the directory where the HTML files are written.

	EPGO_SITE_URL  this is the website URL that the public will use

    EPGO_TEMPLATE_PATH  this is the directory that contains the templates
                   used used to generate the static content of the website.
`

	license = `
%s %s

Copyright (c) 2016, Caltech
All rights not granted herein are expressly reserved by Caltech.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

* Neither the name of epgo nor the names of its
  contributors may be used to endorse or promote products derived from
  this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
`
	showHelp    bool
	showVersion bool
	showLicense bool

	enableSearch bool

	htdocs       string
	dbName       string
	bleveNames   string // NOTE: this is a colon delimited string of index names
	templatePath string
	apiURL       string
	siteURL      string

	indexAlias bleve.IndexAlias
	index      bleve.Index
)

// QueryOptions holds the support query terms expected in either a GET or POST
// to the webserver
type QueryOptions struct {
	// Bleve specific properties
	Explain    bool              `json:"explain"`
	FilterTerm map[string]string `json:"filter_term,omitempty"`
	Type       string            `json:"type,omitempty"`

	// Unified search form properties, works for both Basic and Advanced search
	Method string `json:"method"`
	Action string `json:"action"`

	// This holds the query fields submitted
	Q         string `json:"q"`
	QExact    string `json:"q_exact"`
	QExcluded string `json:"q_excluded"`
	QRequired string `json:"q_required"`
	Size      int    `json:"size"`
	From      int    `json:"from"`
	AllIDs    bool   `json:"all_ids"`

	// Results olds the submitted query results
	Total           int                  `json:"total"`
	DetailsBaseURI  string               `json:"details_base_uri"`
	QueryURLEncoded string               `json:"query_urlencoded"`
	Request         *bleve.SearchRequest `json:"request"`
	Results         *bleve.SearchResult  `json:"results"`
}

// Parses the submitted map for query options setting the internals of the QueryOptions structure
func (q *QueryOptions) Parse(m map[string]interface{}) error {
	var (
		err error
	)
	// raw is a tempory data structure to sanitize the
	// form request submitted via the query.
	raw := new(QueryOptions)
	isQuery := false

	src, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("Can't marshal %+v, %s", m, err)
	}
	err = json.Unmarshal(src, &raw)
	if err != nil {
		return fmt.Errorf("Can't unmarshal %s, %s", src, err)
	}
	if len(raw.Q) > 0 {
		q.Q = raw.Q
		isQuery = true
	}
	if len(raw.QExact) > 0 {
		q.QExact = raw.QExact
		isQuery = true
	}
	if len(raw.QExcluded) > 0 {
		q.QExcluded = q.QExact
	}
	if len(raw.QRequired) > 0 {
		q.QRequired = raw.QRequired
		isQuery = true
	}

	if isQuery == false {
		return fmt.Errorf("Missing query value fields")
	}

	if raw.AllIDs == true {
		q.AllIDs = true
	}

	//Note: if q.Size is not set by the query request pick a nice default value
	if raw.Size <= 1 {
		q.Size = 10
	} else {
		q.Size = raw.Size
	}
	if raw.From < 0 {
		q.From = 0
	} else {
		q.From = raw.From
	}
	return nil
}

func uInt64ToInt(u uint64) (int, error) {
	return strconv.Atoi(fmt.Sprintf("%d", u))
}

// AttachSearchResults sets the value of the SearchResults field in the QueryOptions struct.
func (q *QueryOptions) AttachSearchResults(sr *bleve.SearchResult) {
	if sr != nil {
		q.Results = sr
		q.Total, _ = uInt64ToInt(sr.Total)
		q.Request = sr.Request
	} else {
		q.Total = 0
	}

	v := url.Values{}
	if q.AllIDs == true {
		v.Add("all_ids", "true")
	}
	v.Add("size", fmt.Sprintf("%d", q.Size))
	v.Add("from", fmt.Sprintf("%d", q.From))
	v.Add("total", fmt.Sprintf("%d", q.Total))
	v.Add("q", q.Q)
	v.Add("q_required", q.QRequired)
	v.Add("q_exact", q.QExact)
	v.Add("q_excluded", q.QExcluded)
	q.QueryURLEncoded = v.Encode()
}

func resultsHandler(w http.ResponseWriter, r *http.Request) {
	urlQuery := r.URL.Query()
	err := r.ParseForm()
	if err != nil {
		responseLogger(r, http.StatusBadRequest, err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("error in POST: %s", err)))
		return
	}

	// Collect the submissions fields.
	submission := make(map[string]interface{})
	// Basic Search results
	if r.Method == "GET" {
		for k, v := range urlQuery {
			if k == "all_ids" {
				if b, err := strconv.ParseBool(strings.Join(v, "")); err == nil {
					submission[k] = b
				}
			} else if k == "from" || k == "size" || k == "total" {
				if i, err := strconv.Atoi(strings.Join(v, "")); err == nil {
					submission[k] = i
				}
			} else if k == "q" || k == "q_exact" || k == "q_excluded" || k == "q_required" {
				submission[k] = strings.Join(v, "")
			}
		}
	}

	// Advanced Search results
	if r.Method == "POST" {
		for k, v := range r.Form {
			if k == "all_ids" {
				if b, err := strconv.ParseBool(strings.Join(v, "")); err == nil {
					submission[k] = b
				}
			} else if k == "from" || k == "size" || k == "total" {
				if i, err := strconv.Atoi(strings.Join(v, "")); err == nil {
					submission[k] = i
				}
			} else if k == "q" || k == "q_exact" || k == "q_excluded" || k == "q_required" {
				submission[k] = strings.Join(v, "")
			}
		}
	}

	q := new(QueryOptions)
	err = q.Parse(submission)
	if err != nil {
		responseLogger(r, http.StatusBadRequest, err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}

	//
	// Note: Adding logic to handle basic and advanced search...
	//
	// q           NewQueryStringQuery
	// q_required  NewQueryStringQuery with a + prefix for each strings.Fields(q_required) value
	// q_exact     NewMatchPhraseQuery
	// q_excluded  NewQueryStringQuery with a - prefix for each strings.Feilds(q_excluded) value
	//
	var conQry []bleve.Query

	if q.Q != "" {
		conQry = append(conQry, bleve.NewQueryStringQuery(q.Q))
	}
	if q.QExact != "" {
		conQry = append(conQry, bleve.NewMatchPhraseQuery(q.QExact))
	}
	var terms []string
	for _, s := range strings.Fields(q.QRequired) {
		terms = append(terms, fmt.Sprintf("+%s", strings.TrimSpace(s)))
	}
	for _, s := range strings.Fields(q.QExcluded) {
		terms = append(terms, fmt.Sprintf("-%s", strings.TrimSpace(s)))
	}
	if len(terms) > 0 {
		qString := strings.Join(terms, " ")
		conQry = append(conQry, bleve.NewQueryStringQuery(qString))
	}

	qry := bleve.NewConjunctionQuery(conQry)
	if q.Size == 0 {
		q.Size = 10
	}
	searchRequest := bleve.NewSearchRequestOptions(qry, q.Size, q.From, q.Explain)
	if searchRequest == nil {
		responseLogger(r, http.StatusBadRequest, fmt.Errorf("Can't build new search request options %+v, %s", qry, err))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}

	/*
		EPrintID:     jsonDoc.ID,
		Type:         jsonDoc.Type,
		OfficialURL:  jsonDoc.OfficialURL,
		Title:        jsonDoc.Title,
		Abstract:     jsonDoc.Abstract,
		Keywords:     jsonDoc.Keywords,
		ISSN:         jsonDoc.ISSN,
		Publication:  jsonDoc.Publication,
		Note:         jsonDoc.Note,
		Authors:      jsonDoc.Creators.ToNames(),
		ORCIDs:       jsonDoc.Creators.ToORCIDs(),
		ISNIs:        jsonDoc.Creators.ToISNIs(),
		Rights:       jsonDoc.Rights,
		Funders:      jsonDoc.Funders.ToAgencies(),
		GrantNumbers: jsonDoc.Funders.ToGrantNumbers(),
		PubDate:      jsonDoc.PubDate(),
		LocalGroup:   jsonDoc.LocalGroup,
	*/

	// Return all fields
	searchRequest.Fields = []string{
		"EPrintID",
		"Type",
		"OfficialURL",
		"Title",
		"Abstract",
		"Keywords",
		"ISSN",
		"Publication",
		"Note",
		"Authors",
		"ORCIDs",
		"ISNIs",
		"Rights",
		"Funders",
		"GrantNumbers",
		"PubDate",
		"LocalGroup",
	}

	// Setup Highlights
	searchRequest.Highlight = bleve.NewHighlight()
	for _, fieldName := range searchRequest.Fields {
		searchRequest.Highlight.AddField(fieldName)
	}

	// Setup fieldName
	for _, fieldName := range []string{"Authors", "Keywords", "Funders", "ORCIDs", "ISNIs", "GrantNumbers", "Publication", "LocalGroup"} {
		facet := bleve.NewFacetRequest(fieldName, 3)
		searchRequest.AddFacet(fieldName, facet)
	}

	searchResults, err := indexAlias.Search(searchRequest)
	if err != nil {
		responseLogger(r, http.StatusInternalServerError, fmt.Errorf("Bleve results error %v, %s", qry, err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}

	// q (QueryOptions) performs double duty as both the structure for query submission as well
	// as carring the results to support paging and other types of navigation through
	// the query set. Results are a query with the bleve.SearchReults merged
	q.AttachSearchResults(searchResults)
	pageHTML := path.Join(templatePath, "results.html")
	pageInclude := path.Join(templatePath, "results.include")

	// Load my templates and setup to execute them
	tmpl, err := tmplfn.AssemblePage(pageHTML, pageInclude, epgo.TmplFuncs)
	if err != nil {
		responseLogger(r, http.StatusInternalServerError, fmt.Errorf("Template Errors: %s, %s, %s\n", pageHTML, pageInclude, err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Template errors: %s", err)))
		return
	}

	// Render the page
	w.Header().Set("Content-Type", "text/html")
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, q)
	if err != nil {
		responseLogger(r, http.StatusInternalServerError, fmt.Errorf("Can't render %s, %s, %s", pageHTML, pageInclude, err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Template error"))
		return
	}
	//NOTE: This bit of ugliness is here because I need to allow <mark> elements and ellipis in the results fragments
	w.Write(bytes.Replace(bytes.Replace(bytes.Replace(buf.Bytes(), []byte("&lt;mark&gt;"), []byte("<mark>"), -1), []byte("&lt;/mark&gt;"), []byte("</mark>"), -1), []byte(`…`), []byte(`&hellip;`), -1))
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	//logRequest(r)
	// If GET with Query String or POST pass to results handler
	// else display Basic Search Form
	query := r.URL.Query()
	if r.Method == "POST" || len(query) > 0 {
		resultsHandler(w, r)
		return
	}

	// Shared form data fields for a New Search.
	formData := struct {
		URI string
	}{
		URI: "/",
	}

	// Handle the basic or advanced search form requests.
	var (
		tmpl *template.Template
		err  error
	)
	pageHTML := path.Join(templatePath, "search.html")
	pageInclude := path.Join(templatePath, "search.include")
	w.Header().Set("Content-Type", "text/html")
	if strings.HasPrefix(r.URL.Path, "/search/") == true {
		formData.URI = "/search/"
		tmpl, err = tmplfn.AssemblePage(pageHTML, pageInclude, epgo.TmplFuncs)
		if err != nil {
			fmt.Printf("Can't read search templates %s, %s, %s", pageHTML, pageInclude, err)
			return
		}
	}

	err = tmpl.Execute(w, formData)
	if err != nil {
		responseLogger(r, http.StatusInternalServerError, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}
}

func requestLogger(next http.Handler) http.Handler {
	//FIXME: Need to convert to the common log format: htts://en.wikipedia.org/wiki/Common_Log_Format
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//FIXME: add the response status returned.
		q := r.URL.Query()
		if len(q) > 0 {
			log.Printf("Request: %s Path: %s RemoteAddr: %s UserAgent: %s Query: %+v\n", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent(), q)
		} else {
			log.Printf("Request: %s Path: %s RemoteAddr: %s UserAgent: %s\n", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
		}
		next.ServeHTTP(w, r)
	})
}

func responseLogger(r *http.Request, status int, err error) {
	//FIXME: Need to convert to the common log format: htts://en.wikipedia.org/wiki/Common_Log_Format
	q := r.URL.Query()
	if len(q) > 0 {
		log.Printf("Response: %s Path: %s RemoteAddr: %s UserAgent: %s Query: %+v Status: %d, %s %q\n", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent(), q, status, http.StatusText(status), err)
	} else {
		log.Printf("Response: %s Path: %s RemoteAddr: %s UserAgent: %s Status: %d, %s %q\n", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent(), status, http.StatusText(status), err)
	}
}

// isMultiViewPath checks to see if the path requested behaves like an Apache MultiView request
func isMultiViewPath(p string) bool {
	// check to see if p plus .html extension exists
	fname := fmt.Sprintf("%s.html", p)
	if _, err := os.Stat(path.Join(htdocs, fname)); err == nil {
		return true
	}
	return false
}

func multiViewPath(p string) string {
	return fmt.Sprintf("%s.html", p)
}

func customRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enableSearch == true {
			if strings.HasPrefix(r.URL.Path, "/search/") == true {
				searchHandler(w, r)
				return
			}
		}

		// NOTE: The default static file server doesn't seem send the correct mimetype for RSS and JSON responses.

		// If this is a MultiViews style request (i.e. missing .html) then update r.URL.Path
		if isMultiViewPath(r.URL.Path) == true {
			p := multiViewPath(r.URL.Path)
			r.URL.Path = p
		}
		// If we make it this far, fall back to the default handler
		next.ServeHTTP(w, r)
	})
}

func check(cfg *cli.Config, key, value string) string {
	if value == "" {
		log.Fatal("Missing %s_%s", cfg.EnvPrefix, strings.ToUpper(key))
		return ""
	}
	return value
}

// switchIndex returns the error if a problem happens swaping the index
func switchIndex() error {
	var (
		curName  string
		nextName string
	)
	curName = index.Name()
	if len(curName) == 0 {
		return fmt.Errorf("No index defined")
	}
	indexList := strings.Split(bleveNames, ":")
	if len(indexList) > 1 {
		// Find the name of the next index
		for i, iName := range indexList {
			if strings.Compare(iName, curName) == 0 {
				i++
				// Wrap to the beginning if we go off end of list
				if i >= len(indexList) {
					i = 0
				}
				nextName = indexList[i]
			}
		}
		log.Printf("Switching from %q to %q", curName, nextName)
		indexNext, err := bleve.Open(nextName)
		if err != nil {
			return fmt.Errorf("Can't open Bleve index %q, %s", nextName, err)
		}
		indexAlias.Swap([]bleve.Index{indexNext}, []bleve.Index{index})
		log.Printf("Closing %q", curName)
		index.Close()
		// Point index at indexNext
		index = indexNext
		return nil
	}
	return fmt.Errorf("Only %q index defined, no swap possible", curName)
}

func handleSignals() {
	intChan := make(chan os.Signal, 1)
	signal.Notify(intChan, os.Interrupt)
	go func() {
		for {
			<-intChan
			//handle SIGINT by shutting down servepages
			if index != nil {
				log.Printf("Closing index %q", index.Name())
				index.Close()
			}
			log.Println("SIGINT received, shutting down")
			os.Exit(0)
		}
	}()

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGTERM)
	go func() {
		for {
			<-termChan
			//handle SIGTERM by shutting down servepages
			if index != nil {
				log.Printf("Closing index %q", index.Name())
				index.Close()
			}
			log.Println("SIGTERM received, shutting down")
			os.Exit(0)
		}
	}()

	hupChan := make(chan os.Signal, 1)
	signal.Notify(hupChan, syscall.SIGHUP)
	go func() {
		for {
			<-hupChan
			//NOTE: HUP triggers an swap of indexes used by search
			log.Println("SIGHUP received, swaping index")
			err := switchIndex()
			if err != nil {
				log.Printf("Error swaping index %s", err)
				return
			}
			log.Printf("Active Index is now %q", index.Name())
		}
	}()
/*NOTE: Not available outside BSD-* systems
	infoChan := make(chan os.Signal, 1)
	signal.Notify(infoChan, syscall.SIGINFO)
	go func() {
		for {
			<-infoChan
			//handle SIGINFO, send out the current index being used
			log.Printf("Status: site url is %q", siteURL)
			log.Printf("Status: search index is %q", index.Name())
		}
	}()
*/
	/*
		signalChannel := make(chan os.Signal, 4)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINFO)
		go func() {
			// handle signals for the duration of the process
			for {
				sig := <-signalChannel
				switch sig {
				case os.Interrupt:
					//handle SIGINT by shutting down servepages
					if index != nil {
						log.Printf("Closing index %q", index.Name())
						index.Close()
					}
					log.Println("SIGINT received, shutting down")
					os.Exit(0)
				case syscall.SIGTERM:
					//handle SIGTERM by shutting down servepages
					if index != nil {
						log.Printf("Closing index %q", index.Name())
						index.Close()
					}
					log.Println("SIGTERM received, shutting down")
					os.Exit(0)
				case syscall.SIGINFO:
					//handle SIGINFO, send out the current index being used
					log.Printf("Status: site url is %q", siteURL)
					log.Printf("Status: search index is %q", index.Name())
				case syscall.SIGHUP:
					//NOTE: HUP triggers an swap of indexes used by search
					log.Println("SIGHUP received, swaping index")
					err := switchIndex()
					if err != nil {
						log.Printf("Error swaping index %s", err)
						return
					}
					log.Printf("Active Index is now %q", index.Name())
				}
			}
		}()
	*/

}

func init() {
	// Log to standard out
	log.SetOutput(os.Stdout)

	// Setup options
	flag.BoolVar(&showHelp, "h", false, "display help")
	flag.BoolVar(&showHelp, "help", false, "display help")
	flag.BoolVar(&showVersion, "v", false, "display version")
	flag.BoolVar(&showVersion, "version", false, "display version")
	flag.BoolVar(&showLicense, "l", false, "display license")
	flag.BoolVar(&showLicense, "license", false, "display license")

	// App Specific options
	flag.StringVar(&htdocs, "htdocs", "", "specify where to write the HTML files to")
	flag.StringVar(&bleveNames, "bleve", "", "a colon delimited list of Bleve index db names")
	flag.StringVar(&siteURL, "site-url", "", "the website url")
	flag.StringVar(&templatePath, "template-path", "", "specify where to read the templates from")
	flag.BoolVar(&enableSearch, "enable-search", true, "turn on search support in webserver")
}

func main() {
	var (
		err error
	)

	appName := path.Base(os.Args[0])
	cfg := cli.New(appName, "EPGO", fmt.Sprintf(license, appName, epgo.Version), epgo.Version)
	cfg.UsageText = fmt.Sprintf(usage, appName)
	cfg.DescriptionText = fmt.Sprintf(description, appName, appName)
	cfg.OptionsText = "OPTIONS\n"

	flag.Parse()
	if showHelp == true {
		fmt.Println(cfg.Usage())
		os.Exit(0)
	}
	if showVersion == true {
		fmt.Println(cfg.Version())
		os.Exit(0)
	}
	if showLicense == true {
		fmt.Println(cfg.License())
		os.Exit(0)
	}

	// Check to see we can merge the required fields are merged.
	htdocs = check(cfg, "htdocs", cfg.MergeEnv("htdocs", htdocs))
	templatePath = check(cfg, "template_path", cfg.MergeEnv("template_path", templatePath))
	siteURL = check(cfg, "site_url", cfg.MergeEnv("site_url", siteURL))
	bleveNames = check(cfg, "bleve", cfg.MergeEnv("BLEVE", bleveNames))

	if htdocs != "" {
		if _, err := os.Stat(htdocs); os.IsNotExist(err) {
			os.MkdirAll(htdocs, 0775)
		}
	}

	//NOTE: Need to get hostname and port from siteURL
	u, err := url.Parse(siteURL)
	if err != nil {
		log.Fatal(err)
	}

	handleSignals()

	//
	// Run the webserver and search service
	//
	log.Printf("%s %s\n", appName, epgo.Version)

	if enableSearch == true {
		// Wake up our search engine
		indexList := strings.Split(bleveNames, ":")
		availableIndex := false
		for i := 0; i < len(indexList) && availableIndex == false; i++ {
			indexName := indexList[i]
			log.Printf("Opening %q", indexName)
			index, err = bleve.Open(indexName)
			if err != nil {
				log.Printf("Can't open Bleve index %q, %s, trying next index", indexName, err)
			} else {
				indexAlias = bleve.NewIndexAlias(index)
				availableIndex = true
			}
		}
		if availableIndex == false {
			log.Fatalf("No index available %s", bleveNames)
		}
		defer index.Close()
	}

	// Send static file request to the default handler,
	// search routes are handled by middleware customRoutes()
	log.Printf("Adding handler for %q", htdocs)
	http.Handle("/", http.FileServer(http.Dir(htdocs)))

	log.Printf("Listening on %s\n", u.String())
	err = http.ListenAndServe(u.Host, requestLogger(customRoutes(http.DefaultServeMux)))
	if err != nil {
		log.Fatal(err)
	}
}
