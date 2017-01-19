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
package epgo

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"

	// Caltech Library packages
	"github.com/boltdb/bolt"
	"github.com/caltechlibrary/bibtex"
	"github.com/caltechlibrary/cli"
	"github.com/caltechlibrary/tmplfn"
)

// These are our main bucket and index buckets
var (
	// Primary collection
	ePrintBucket = []byte("eprints")

	// Indexes available
	indexDelimiter   = "|"
	pubDatesBucket   = []byte("publicationDates")
	localGroupBucket = []byte("localGroup")
	orcidBucket      = []byte("orcid") // NOTE: We can probably combined bucket for ORCID or ISNI ids

	//FIXME: Additional indexes might be useful.
	// publicationsBucket  = []byte("publications")
	// titlesBucket        = []byte("titles")
	// subjectsBucket      = []byte("subjects")
	// authors             = []byte("authors")
	// additionDatesBucket = []byte("additionsDates")

	// TmplFuncs is the collected functions available in EPGO templates
	TmplFuncs = tmplfn.Join(tmplfn.TimeMap, tmplfn.PageMap)
)

func failCheck(err error, msg string) {
	if err != nil {
		log.Fatalf("%s\n", msg)
	}
}

// EPrintsAPI holds the basic connectin information to read the REST API for EPrints
type EPrintsAPI struct {
	XMLName        xml.Name `json:"-"`
	URL            *url.URL `xml:"epgo>api_url" json:"api_url"`                 // EPGO_API_URL
	Dataset        string   `xml:"epgo>dataset" json:"dataset"`                 // EPGO_DATASET
	BleveName      string   `xml:"epgo>bleve" json:"bleve"`                     // EPGO_BLEVE
	Htdocs         string   `xml:"epgo>htdocs" json:"htdocs"`                   // EPGO_HTDOCS
	TemplatePath   string   `xml:"epgo>template_path" json:"template_path"`     // EPGO_TEMPLATES
	SiteURL        *url.URL `xml:"epgo>site_url" json:"site_url"`               // EPGO_SITE_URL
	RepositoryPath string   `xml:"epgo>repository_path" json:"repository_path"` // EPGO_REPOSITORY_PATH
}

// Person returns the contents of eprint>creators>item>name as a struct
type Person struct {
	XMLName xml.Name `json:"-"`
	Given   string   `xml:"name>given" json:"given"`
	Family  string   `xml:"name>family" json:"family"`
	ID      string   `xml:"id,omitempty" json:"id"`
	ORCID   string   `xml:"orcid,omitempty" json:"orcid"`
	ISNI    string   `xml:"isni,omitempty" json:"isni"`
}

// PersonList is an array of pointers to Person structs
type PersonList []*Person

// RelatedURL is a structure containing information about a relationship
type RelatedURL struct {
	XMLName     xml.Name `json:"-"`
	URL         string   `xml:"url" json:"url"`
	Type        string   `xml:"type" json:"type"`
	Description string   `xml:"description" json:"description"`
}

// NumberingSystem is a structure describing other numbering systems for record
type NumberingSystem struct {
	XMLName xml.Name `json:"-"`
	Name    string   `xml:"name" json:"name"`
	ID      string   `xml:"id" json:"id"`
}

// Funder is a structure describing a funding source for record
type Funder struct {
	XMLName     xml.Name `json:"-"`
	Agency      string   `xml:"agency" json:"agency"`
	GrantNumber string   `xml:"grant_number,omitempty" json:"grant_number"`
}

// FunderList is an array of pointers to Funder structs
type FunderList []*Funder

// File structures in Document
type File struct {
	XMLName   xml.Name `json:"-"`
	ID        string   `xml:"id,attr" json:"id"`
	FileID    int      `xml:"fileid" json:"fileid"`
	DatasetID string   `xml:"datasetid" json:"datasetid"`
	ObjectID  int      `xml:"objectid" json:"objectid"`
	Filename  string   `xml:"filename" json:"filename"`
	MimeType  string   `xml:"mime_type" json:"mime_type"`
	Hash      string   `xml:"hash" json:"hash"`
	HashType  string   `xml:"hash_type" json:"hash_type"`
	FileSize  int      `xml:"filesize" json:"filesize"`
	MTime     string   `xml:"mtime" json:"mtime"`
	URL       string   `xml:"url" json:"url"`
}

// Document structures in Record
type Document struct {
	XMLName   xml.Name `json:"-"`
	ID        string   `xml:"id,attr" json:"id"`
	DocID     int      `xml:"docid" json:"docid"`
	RevNumber int      `xml:"rev_number" json:"rev_number"`
	Files     []*File  `xml:"files>file" json:"files"`
	EPrintID  int      `xml:"eprintid" json:"eprintid"`
	Pos       int      `xml:"pos" json:"pos"`
	Placement int      `xml:"placement" json:"placement"`
	MimeType  string   `xml:"mime_type" json:"mime_type"`
	Format    string   `xml:"format" json:"format"`
	Language  string   `xml:"language" json:"language"`
	Security  string   `xml:"security" json:"security"`
	License   string   `xml:"license" json:"license"`
	Main      string   `xml:"main" json:"main"`
	Content   string   `xml:"content" json:"content"`
}

// DocumentList is an array of pointers to Document structs
type DocumentList []*Document

// Record returns a structure that can be converted to JSON easily
type Record struct {
	XMLName              xml.Name           `json:"-"`
	Title                string             `xml:"eprint>title" json:"title"`
	URI                  string             `json:"uri"`
	Abstract             string             `xml:"eprint>abstract" json:"abstract"`
	Documents            DocumentList       `xml:"eprint>documents>document" json:"documents"`
	Note                 string             `xml:"eprint>note" json:"note"`
	ID                   int                `xml:"eprint>eprintid" json:"id"`
	RevNumber            int                `xml:"eprint>rev_number" json:"rev_number"`
	UserID               int                `xml:"eprint>userid" json:"userid"`
	Dir                  string             `xml:"eprint>dir" json:"eprint_dir"`
	Datestamp            string             `xml:"eprint>datestamp" json:"datestamp"`
	LastModified         string             `xml:"eprint>lastmod" json:"lastmod"`
	StatusChange         string             `xml:"eprint>status_changed" json:"status_changed"`
	Type                 string             `xml:"eprint>type" json:"type"`
	MetadataVisibility   string             `xml:"eprint>metadata_visibility" json:"metadata_visibility"`
	Creators             PersonList         `xml:"eprint>creators>item" json:"creators"`
	IsPublished          string             `xml:"eprint>ispublished" json:"ispublished"`
	Subjects             []string           `xml:"eprint>subjects>item" json:"subjects"`
	FullTextStatus       string             `xml:"eprint>full_text_status" json:"full_text_status"`
	Keywords             string             `xml:"eprint>keywords" json:"keywords"`
	Date                 string             `xml:"eprint>date" json:"date"`
	DateType             string             `xml:"eprint>date_type" json:"date_type"`
	Publication          string             `xml:"eprint>publication" json:"publication"`
	Volume               string             `xml:"eprint>volume" json:"volume"`
	Number               string             `xml:"eprint>number" json:"number"`
	PageRange            string             `xml:"eprint>pagerange" json:"pagerange"`
	IDNumber             string             `xml:"eprint>id_number" json:"id_number"`
	Referred             bool               `xml:"eprint>refereed" json:"refereed"`
	ISSN                 string             `xml:"eprint>issn" json:"issn"`
	OfficialURL          string             `xml:"eprint>official_url" json:"official_url"`
	RelatedURL           []*RelatedURL      `xml:"eprint>related_url>item" json:"related_url"`
	ReferenceText        []string           `xml:"eprint>referencetext>item" json:"referencetext"`
	Rights               string             `xml:"eprint>rights" json:"rights"`
	OfficialCitation     string             `xml:"eprint>official_cit" json:"official_citation"`
	OtherNumberingSystem []*NumberingSystem `xml:"eprint>other_numbering_system>item,omitempty" json:"other_numbering_system"`
	Funders              FunderList         `xml:"eprint>funders>item" json:"funders"`
	Collection           string             `xml:"eprint>collection" json:"collection"`
	Reviewer             string             `xml:"eprint>reviewer" json:"reviewer"`
	LocalGroup           []string           `xml:"eprint>local_group>item" json:"local_group"`
}

type ePrintIDs struct {
	XMLName xml.Name `xml:"html" json:"-"`
	IDs     []string `xml:"body>ul>li>a" json:"ids"`
}

func normalizeDate(in string) string {
	parts := strings.Split(in, "-")
	if len(parts) == 1 {
		parts = append(parts, "01")
		parts = append(parts, "01")
	}
	if len(parts) == 2 {
		parts = append(parts, "01")
	}
	for i := 0; i < len(parts); i++ {
		x, err := strconv.Atoi(parts[i])
		if err != nil {
			x = 1
		}
		if i == 0 {
			parts[i] = fmt.Sprintf("%0.4d", x)
		} else {
			parts[i] = fmt.Sprintf("%0.2d", x)
		}
	}
	return strings.Join(parts, "-")
}

// ToBibTeXElement takes an epgo.Record and turns it into a bibtex.Element record
func (rec *Record) ToBibTeXElement() *bibtex.Element {
	bib := &bibtex.Element{}
	bib.Set("type", rec.Type)
	bib.Set("id", fmt.Sprintf("eprint-%d", rec.ID))
	bib.Set("title", rec.Title)
	if len(rec.Abstract) > 0 {
		bib.Set("abstract", rec.Abstract)
	}
	if rec.DateType == "pub" {
		dt, err := time.Parse("2006-01-02", rec.Date)
		if err != nil {
			bib.Set("year", dt.Format("2006"))
			bib.Set("month", dt.Format("January"))
		}
	}
	if len(rec.PageRange) > 0 {
		bib.Set("pages", rec.PageRange)
	}
	/*
		if len(rec.Note) > 0 {
			bib.Set("note", rec.Note)
		}
	*/
	if len(rec.Creators) > 0 {
		people := []string{}
		for _, person := range rec.Creators {
			people = append(people, fmt.Sprintf("%s, %s", person.Family, person.Given))
		}
		bib.Set("author", strings.Join(people, " and "))
	}
	switch rec.Type {
	case "article":
		bib.Set("journal", rec.Publication)
	case "book":
		bib.Set("publisher", rec.Publication)
	}
	if len(rec.Volume) > 0 {
		bib.Set("volume", rec.Volume)
	}
	if len(rec.Number) > 0 {
		bib.Set("number", rec.Number)
	}
	return bib
}

// New creates a new API instance
func New(cfg *cli.Config) (*EPrintsAPI, error) {
	var err error
	apiURL := cfg.Get("api_url")
	siteURL := cfg.Get("site_url")
	htdocs := cfg.Get("htdocs")
	dataset := cfg.Get("dataset")
	bleveName := cfg.Get("bleve")
	templatePath := cfg.Get("template_path")
	repositoryPath := cfg.Get("repository_path")

	if apiURL == "" {
		return nil, fmt.Errorf("Environment not configured, missing eprint api url")
	}
	api := new(EPrintsAPI)
	api.URL, err = url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("api url is malformed %s, %s", apiURL, err)
	}
	api.SiteURL, err = url.Parse(siteURL)
	if err != nil {
		return nil, fmt.Errorf("site url malformed %s, %s", siteURL, err)
	}
	if htdocs == "" {
		htdocs = "htdocs"
	}
	if datasetName == "" {
		datasetName = "eprints.boltdb"
	}
	if bleveName == "" {
		bleveName = "eprints.bleve"
	}
	if templatePath == "" {
		templatePath = "templates"
	}
	if repositoryPath == "" {
		repositoryPath = "repository"
	}
	api.Htdocs = htdocs
	api.Dataset = dataset
	api.TemplatePath = templatePath
	api.BleveName = bleveName
	api.RepositoryPath = repositoryPath
	return api, nil
}

type byURI []string

func (s byURI) Len() int {
	return len(s)
}

func (s byURI) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byURI) Less(i, j int) bool {
	s1 := strings.TrimSuffix(path.Base(s[i]), path.Ext(s[i]))
	s2 := strings.TrimSuffix(path.Base(s[j]), path.Ext(s[j]))
	a1, err := strconv.Atoi(s1)
	if err != nil {
		return false
	}
	a2, err := strconv.Atoi(s2)
	if err != nil {
		return false
	}
	return a1 > a2
}

// ListEPrintsURI returns a list of eprint record ids
func (api *EPrintsAPI) ListEPrintsURI() ([]string, error) {
	var (
		results []string
	)

	api.URL.Path = path.Join("rest", "eprint") + "/"
	resp, err := http.Get(api.URL.String())
	if err != nil {
		return nil, fmt.Errorf("requested %s, %s", api.URL.String(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error %s, %s", api.URL.String(), resp.Status)
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("content can't be read %s, %s", api.URL.String(), err)
	}
	eIDs := new(ePrintIDs)
	err = xml.Unmarshal(content, &eIDs)
	if err != nil {
		return nil, err
	}
	// Build a list of Unique IDs in a map, then convert unique querys to results array
	m := make(map[string]bool)
	for _, val := range eIDs.IDs {
		if strings.HasSuffix(val, ".xml") == true {
			uri := "/" + path.Join("rest", "eprint", val)
			if _, hasID := m[uri]; hasID == false {
				// Save the new ID found
				m[uri] = true
				// Only store Unique IDs in result
				results = append(results, uri)
			}
		}
	}
	return results, nil
}

// GetEPrint retrieves an EPrint record via REST API and returns a Record structure and error
func (api *EPrintsAPI) GetEPrint(uri string) (*Record, error) {
	api.URL.Path = uri
	resp, err := http.Get(api.URL.String())
	if err != nil {
		return nil, fmt.Errorf("requested %s, %s", api.URL.String(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http error %s, %s", api.URL.String(), resp.Status)
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("content can't be read %s, %s", api.URL.String(), err)
	}
	rec := new(Record)
	err = xml.Unmarshal(content, &rec)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// ToNames takes an array of pointers to Person and returns a list of names (family, given)
func (persons PersonList) ToNames() []string {
	var result []string

	for _, person := range persons {
		result = append(result, fmt.Sprintf("%s, %s", person.Family, person.Given))
	}
	return result
}

// ToORCIDs takes an an array of pointers to Person and returns a list of ORCID ids
func (persons PersonList) ToORCIDs() []string {
	var result []string

	for _, person := range persons {
		result = append(result, person.ORCID)
	}

	return result
}

// ToISNIs takes an array of pointers to Person and returns a list of ISNI ids
func (persons PersonList) ToISNIs() []string {
	var result []string

	for _, person := range persons {
		result = append(result, person.ISNI)
	}

	return result
}

// ToAgencies takes an array of pointers to Funders and returns a list of Agency names
func (funders FunderList) ToAgencies() []string {
	var result []string

	for _, funder := range funders {
		result = append(result, funder.Agency)
	}

	return result
}

// ToGrantNumbers takes an array of pointers to Funders and returns a list of Agency names
func (funders FunderList) ToGrantNumbers() []string {
	var result []string

	for _, funder := range funders {
		result = append(result, funder.GrantNumber)
	}

	return result
}

func (record *Record) PubDate() string {
	if strings.Compare(record.DateType, "published") == 0 {
		return record.Date
	}
	return ""
}

// initBuckets initializes expected buckets if necessary for boltdb
func initBuckets(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(ePrintBucket); err != nil {
			return fmt.Errorf("create bucket %s: %s", ePrintBucket, err)
		}
		if _, err := tx.CreateBucketIfNotExists(pubDatesBucket); err != nil {
			return fmt.Errorf("create bucket %s: %s", pubDatesBucket, err)
		}
		if _, err := tx.CreateBucketIfNotExists(localGroupBucket); err != nil {
			return fmt.Errorf("create bucket %s: %s", localGroupBucket, err)
		}
		if _, err := tx.CreateBucketIfNotExists(orcidBucket); err != nil {
			return fmt.Errorf("create bucket %s: %s", orcidBucket, err)
		}
		return nil
	})

}

// ListURI returns a list of eprint record ids from the database
func (api *EPrintsAPI) ListURI(start, count int) ([]string, error) {
	var results []string
	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("ListURI %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		recs := tx.Bucket(ePrintBucket)
		c := recs.Cursor()
		p := 0
		if count < 0 {
			bStats := recs.Stats()
			count = bStats.KeyN
		}
		for uri, _ := c.First(); uri != nil && count > 0; uri, _ = c.Next() {
			if p >= start {
				results = append(results, string(uri))
				count--
			}
			p++
		}
		return nil
	})
	return results, err
}

// Get retrieves an EPrint record from the database
func (api *EPrintsAPI) Get(uri string) (*Record, error) {
	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("Get(%q) %s failed to open db, %s", uri, api.Dataset, err))
	defer db.Close()

	record := new(Record)
	err = db.View(func(tx *bolt.Tx) error {
		recs := tx.Bucket(ePrintBucket)
		src := recs.Get([]byte(uri))
		err := json.Unmarshal(src, &record)
		if err != nil {
			return err
		}
		return nil
	})
	return record, err
}

// GetAllRecordIDs reads and returns a list of record ids found.
func (api *EPrintsAPI) GetAllRecordIDs(direction int) ([]string, error) {
	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetAllRecordIDs() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	//	var records []Record
	var (
		results []string
	)
	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			idx := tx.Bucket(pubDatesBucket)
			c := idx.Cursor()
			for k, uri := c.First(); k != nil; k, uri = c.Next() {
				results = append(results, fmt.Sprintf("%s", uri))
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			idx := tx.Bucket(pubDatesBucket)
			c := idx.Cursor()
			for k, uri := c.Last(); k != nil; k, uri = c.Prev() {
				results = append(results, fmt.Sprintf("%s", uri))
			}
			return nil
		})
	}
	return results, err
}

// GetAllRecords reads and returns all records keys
// returning an array of keys in  ascending or decending order
func (api *EPrintsAPI) GetAllRecords(direction int) ([]*Record, error) {
	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetAllRecords() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	//	var records []Record
	var (
		results []*Record
	)
	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(pubDatesBucket)
			c := idx.Cursor()
			for k, uri := c.First(); k != nil; k, uri = c.Next() {
				rec := new(Record)
				src := recs.Get([]byte(uri))
				err := json.Unmarshal(src, rec)
				if err != nil {
					return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
				}
				results = append(results, rec)
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(pubDatesBucket)
			c := idx.Cursor()
			for k, uri := c.Last(); k != nil; k, uri = c.Prev() {
				rec := new(Record)
				src := recs.Get([]byte(uri))
				err := json.Unmarshal(src, rec)
				if err != nil {
					return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
				}
				results = append(results, rec)
			}
			return nil
		})
	}
	return results, err
}

// GetPublications reads the index for published content and returns a populated
// array of records found in index in ascending or decending order
func (api *EPrintsAPI) GetPublications(start, count, direction int) ([]*Record, error) {
	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetPulishedRecords() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	//	var records []Record
	var (
		results []*Record
	)
	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(pubDatesBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.First(); k != nil && count > 0; k, uri = c.Next() {
				if p >= start {
					rec := new(Record)
					src := recs.Get([]byte(uri))
					err := json.Unmarshal(src, rec)
					if err != nil {
						return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
					}
					if rec.IsPublished == "pub" {
						results = append(results, rec)
						count--
					}
				}
				p++
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(pubDatesBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.Last(); k != nil && count > 0; k, uri = c.Prev() {
				if p >= start {
					rec := new(Record)
					src := recs.Get([]byte(uri))
					err := json.Unmarshal(src, rec)
					if err != nil {
						return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
					}
					if rec.IsPublished == "pub" {
						results = append(results, rec)
						count--
					}
				}
				p++
			}
			return nil
		})
	}
	return results, err
}

// GetArticles reads the index for published content and returns a populated
// array of records found in index in decending order
func (api *EPrintsAPI) GetArticles(start, count, direction int) ([]*Record, error) {
	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetArticles() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	//	var records []Record
	var (
		results []*Record
	)
	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(pubDatesBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.First(); k != nil && count > 0; k, uri = c.Next() {
				if p >= start {
					rec := new(Record)
					src := recs.Get([]byte(uri))
					err := json.Unmarshal(src, rec)
					if err != nil {
						return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
					}
					if rec.Type == "article" && rec.IsPublished == "pub" {
						results = append(results, rec)
						count--
					}
				}
				p++
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(pubDatesBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.Last(); k != nil && count > 0; k, uri = c.Prev() {
				if p >= start {
					rec := new(Record)
					src := recs.Get([]byte(uri))
					err := json.Unmarshal(src, rec)
					if err != nil {
						return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
					}
					if rec.Type == "article" && rec.IsPublished == "pub" {
						results = append(results, rec)
						count--
					}
				}
				p++
			}
			return nil
		})
	}
	return results, err
}

// Utility methods used by the LocalGroup and ORCID index related functions
func appendToList(list []string, term string) []string {
	for _, element := range list {
		if strings.Compare(element, term) == 0 {
			return list
		}
	}
	return append(list, term)
}

func firstTerm(s, delimiter string) string {
	r := strings.SplitN(s, delimiter, 2)
	if len(r) > 0 {
		return strings.TrimSpace(r[0])
	}
	return ""
}

// Turn a string into a URL friendly path part
func Slugify(s string) string {
	return url.QueryEscape(s)
}

// GetLocalGroups returns a JSON list of unique Group names in index
func (api *EPrintsAPI) GetLocalGroups(start, count, direction int) ([]string, error) {
	groupNames := []string{}
	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetLocalGroups() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			idx := tx.Bucket(localGroupBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, _ := c.First(); k != nil && count > 0; k, _ = c.Next() {
				if p >= start {
					grp := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					groupNames = appendToList(groupNames, grp)
					count--
				}
				p++
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			idx := tx.Bucket(localGroupBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, _ := c.Last(); k != nil && count > 0; k, _ = c.Prev() {
				if p >= start {
					grp := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					groupNames = appendToList(groupNames, grp)
					count--
				}
				p++
			}
			return nil
		})
	}
	if err != nil {
		return groupNames, err
	}
	return groupNames, nil
}

// GetLocalGroupPublications returns a list of EPrint records with groupName
func (api *EPrintsAPI) GetLocalGroupPublications(groupName string, start, count, direction int) ([]*Record, error) {
	results := []*Record{}

	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetLocalGroupPublications() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(localGroupBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.First(); k != nil && count > 0; k, uri = c.Next() {
				if p >= start {
					grp := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					if strings.Compare(grp, groupName) == 0 {
						rec := new(Record)
						src := recs.Get([]byte(uri))
						err := json.Unmarshal(src, rec)
						if err != nil {
							return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
						}
						results = append(results, rec)
						count--
					}
				}
				p++
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(localGroupBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.Last(); k != nil && count > 0; k, uri = c.Prev() {
				if p >= start {
					grp := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					if strings.Compare(grp, groupName) == 0 {
						rec := new(Record)
						src := recs.Get([]byte(uri))
						err := json.Unmarshal(src, rec)
						if err != nil {
							return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
						}
						results = append(results, rec)
						count--
					}
				}
				p++
			}
			return nil
		})
	}
	if err != nil {
		return results, err
	}
	return results, nil
}

// GetLocalGroupArticles returns a list of EPrint records with groupName
func (api *EPrintsAPI) GetLocalGroupArticles(groupName string, start, count, direction int) ([]*Record, error) {
	results := []*Record{}

	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetLocalGroupArticles() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(localGroupBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.First(); k != nil && count > 0; k, uri = c.Next() {
				if p >= start {
					grp := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					if strings.Compare(grp, groupName) == 0 {
						rec := new(Record)
						src := recs.Get([]byte(uri))
						err := json.Unmarshal(src, rec)
						if err != nil {
							return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
						}
						if rec.Type == "article" && rec.IsPublished == "pub" {
							results = append(results, rec)
							count--
						}
					}
				}
				p++
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(localGroupBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.Last(); k != nil && count > 0; k, uri = c.Prev() {
				if p >= start {
					grp := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					if strings.Compare(grp, groupName) == 0 {
						rec := new(Record)
						src := recs.Get([]byte(uri))
						err := json.Unmarshal(src, rec)
						if err != nil {
							return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
						}
						if rec.Type == "article" && rec.IsPublished == "pub" {
							results = append(results, rec)
							count--
						}
					}
				}
				p++
			}
			return nil
		})
	}
	if err != nil {
		return results, err
	}
	return results, nil
}

// GetORCIDs (or ISNI) returns a list unique of ORCID/ISNI IDs in index
func (api *EPrintsAPI) GetORCIDs(start, count, direction int) ([]string, error) {
	ids := []string{}
	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetORCIDs() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			idx := tx.Bucket(orcidBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, _ := c.First(); k != nil && count > 0; k, _ = c.Next() {
				if p >= start {
					id := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					ids = appendToList(ids, id)
					count--
				}
				p++
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			idx := tx.Bucket(orcidBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, _ := c.Last(); k != nil && count > 0; k, _ = c.Prev() {
				if p >= start {
					id := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					ids = appendToList(ids, id)
					count--
				}
				p++
			}
			return nil
		})
	}
	if err != nil {
		return ids, err
	}
	return ids, nil
}

// GetORCIDPublications returns a list of EPrint records with a given ORCID
func (api *EPrintsAPI) GetORCIDPublications(orcid string, start, count, direction int) ([]*Record, error) {
	results := []*Record{}

	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetORCIDPublications() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(orcidBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.First(); k != nil && count > 0; k, uri = c.Next() {
				if p >= start {
					term := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					if strings.Compare(term, orcid) == 0 {
						rec := new(Record)
						src := recs.Get([]byte(uri))
						err := json.Unmarshal(src, rec)
						if err != nil {
							return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
						}
						results = append(results, rec)
						count--
					}
				}
				p++
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(orcidBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.Last(); k != nil && count > 0; k, uri = c.Prev() {
				if p >= start {
					term := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					if strings.Compare(term, orcid) == 0 {
						rec := new(Record)
						src := recs.Get([]byte(uri))
						err := json.Unmarshal(src, rec)
						if err != nil {
							return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
						}
						results = append(results, rec)
						count--
					}
				}
				p++
			}
			return nil
		})
	}
	if err != nil {
		return results, err
	}
	return results, nil
}

// GetORCIDArticles returns a list of EPrint records with a given ORCID
func (api *EPrintsAPI) GetORCIDArticles(orcid string, start, count, direction int) ([]*Record, error) {
	results := []*Record{}

	db, err := bolt.Open(api.Dataset, 0660, &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true})
	failCheck(err, fmt.Sprintf("GetORCIDArticles() %s failed to open db, %s", api.Dataset, err))
	defer db.Close()

	switch direction {
	case Ascending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(orcidBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.First(); k != nil && count > 0; k, uri = c.Next() {
				if p >= start {
					term := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					if strings.Compare(term, orcid) == 0 {
						rec := new(Record)
						src := recs.Get([]byte(uri))
						err := json.Unmarshal(src, rec)
						if err != nil {
							return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
						}
						if rec.Type == "article" && rec.IsPublished == "pub" {
							results = append(results, rec)
							count--
						}
					}
				}
				p++
			}
			return nil
		})
	case Descending:
		err = db.View(func(tx *bolt.Tx) error {
			recs := tx.Bucket(ePrintBucket)
			idx := tx.Bucket(orcidBucket)
			c := idx.Cursor()
			p := 0
			if count < 0 {
				bStats := idx.Stats()
				count = bStats.KeyN
			}
			for k, uri := c.Last(); k != nil && count > 0; k, uri = c.Prev() {
				if p >= start {
					term := firstTerm(fmt.Sprintf("%s", k), indexDelimiter)
					if strings.Compare(term, orcid) == 0 {
						rec := new(Record)
						src := recs.Get([]byte(uri))
						err := json.Unmarshal(src, rec)
						if err != nil {
							return fmt.Errorf("Can't unmarshal %s, %s", uri, err)
						}
						if rec.Type == "article" && rec.IsPublished == "pub" {
							results = append(results, rec)
							count--
						}
					}
				}
				p++
			}
			return nil
		})
	}
	if err != nil {
		return results, err
	}
	return results, nil
}

// RenderEPrint writes a single EPrint record to disc.
func (api *EPrintsAPI) RenderEPrint(basepath string, record *Record) error {
	// Convert record to JSON
	src, err := json.Marshal(record)
	if err != nil {
		return err
	}
	fname := path.Join(basepath, fmt.Sprintf("%d.json", record.ID))
	return ioutil.WriteFile(fname, src, 0664)
	// FIXME: look at adding other presententations, e.g. HTML, HTML include, BibTeX
}

// RenderDocuments writes JSON, HTML, include and rss to the directory indicated by docpath
func (api *EPrintsAPI) RenderDocuments(docTitle, docDescription, docpath string, records []*Record) error {
	// Create the the directory part of docpath if neccessary
	if _, err := os.Open(path.Join(api.Htdocs, docpath)); err != nil && os.IsNotExist(err) == true {
		os.MkdirAll(path.Join(api.Htdocs, path.Dir(docpath)), 0775)
	}

	//NOTE: create a data wrapper for HTML page creation
	pageData := &struct {
		Version        string
		Basepath       string
		ApiURL         string
		SiteURL        string
		DocTitle       string
		DocDescription string
		Records        []*Record
	}{
		Version:        Version,
		Basepath:       docpath,
		ApiURL:         api.URL.String(),
		SiteURL:        api.SiteURL.String(),
		DocTitle:       docTitle,
		DocDescription: docDescription,
		Records:        records,
	}

	// Writing JSON file
	fname := path.Join(api.Htdocs, docpath+".json")
	src, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("Can't convert records to JSON %s, %s", fname, err)
	}
	err = ioutil.WriteFile(fname, src, 0664)
	if err != nil {
		return fmt.Errorf("Can't write %s, %s", fname, err)
	}

	// Write out RSS 2.0 file
	fname = path.Join(api.TemplatePath, "rss.xml")
	rss20, err := ioutil.ReadFile(fname)
	if err != nil {
		return fmt.Errorf("Can't open template %s, %s", fname, err)
	}
	rssTmpl, err := template.New("rss").Funcs(TmplFuncs).Parse(string(rss20))
	if err != nil {
		return fmt.Errorf("Can't convert records to RSS %s, %s", fname, err)
	}
	fname = path.Join(api.Htdocs, docpath) + ".rss"
	out, err := os.Create(fname)
	if err != nil {
		return fmt.Errorf("Can't write %s, %s", fname, err)
	}
	if err := rssTmpl.Execute(out, pageData); err != nil {
		return fmt.Errorf("Can't render %s, %s", fname, err)
	}
	out.Close()

	// FIXME: Write out BibTeX file.
	bibDoc := []string{}
	for _, rec := range records {
		bibDoc = append(bibDoc, rec.ToBibTeXElement().String())
	}
	fname = path.Join(api.Htdocs, docpath+".bib")
	err = ioutil.WriteFile(fname, []byte(strings.Join(bibDoc, "\n\n")), 0664)
	if err != nil {
		return fmt.Errorf("Can't write %s, %s", fname, err)
	}

	// Write out include file
	fname = path.Join(api.TemplatePath, "page.include")
	pageInclude, err := ioutil.ReadFile(fname)
	if err != nil {
		return fmt.Errorf("Can't open template %s, %s", fname, err)
	}
	pageIncludeTmpl, err := template.New("page.include").Funcs(TmplFuncs).Parse(string(pageInclude))
	if err != nil {
		return fmt.Errorf("Can't parse %s, %s", fname, err)
	}
	fname = path.Join(api.Htdocs, docpath+".include")
	out, err = os.Create(fname)
	if err != nil {
		return fmt.Errorf("Can't write %s, %s", fname, err)
	}
	log.Printf("Writing %s", fname)
	if err := pageIncludeTmpl.Execute(out, pageData); err != nil {
		return fmt.Errorf("Can't render %s, %s", fname, err)
	}
	out.Close()

	pageHTMLTmpl, err := template.New("page.html").Funcs(TmplFuncs).ParseFiles(
		path.Join(api.TemplatePath, "page.include"),
		path.Join(api.TemplatePath, "page.html"),
	)
	if err != nil {
		return fmt.Errorf("Can't parse %s, %s", fname, err)
	}
	fname = path.Join(api.Htdocs, docpath+".html")
	out, err = os.Create(fname)
	if err != nil {
		return fmt.Errorf("Can't write %s, %s", fname, err)
	}

	log.Printf("Writing %s", fname)
	if err := pageHTMLTmpl.Execute(out, pageData); err != nil {
		return fmt.Errorf("Can't render %s, %s", fname, err)
	}
	out.Close()

	return nil
}

// BuildPages generates webpages based on the contents of the exported EPrints data.
// The site builder needs to know the name of the BoltDB, the root directory
// for the website and directory to find the templates
func (api *EPrintsAPI) BuildPages(feedSize int, title, target string, filter func(*EPrintsAPI, int, int, int) ([]*Record, error)) error {
	if feedSize < 1 {
		feedSize = DefaultFeedSize
	}
	// Collect the published records
	docPath := path.Join(api.Htdocs, target)
	log.Printf("Building %s", docPath)
	records, err := filter(api, 0, feedSize, Descending)
	if err != nil {
		return fmt.Errorf("Can't get records for %q %s, %s", title, docPath, err)
	}
	if len(records) == 0 {
		log.Printf("No records found for %q %s", title, docPath)
	} else {
		log.Printf("%d records found.", len(records))
	}
	if err := api.RenderDocuments(title, fmt.Sprintf("Building pages 0 to %d descending", feedSize), target, records); err != nil {
		return fmt.Errorf("%q %s error, %s", title, docPath, err)
	}
	return nil
}

func (api *EPrintsAPI) BuildEPrintMirror() error {
	// checkPath checks  and creates a path if needed
	checkPath := func(p string) error {
		_, err := os.Stat(p)
		if os.IsExist(err) == true {
			return nil
		}
		return os.MkdirAll(p, 0775)
	}

	ids, err := api.GetAllRecordIDs(Descending)
	if err != nil {
		return err
	}

	// Setup subdirs to hold all the individual eprint records.
	keys := []string{}
	subdir := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	q := len(subdir)
	// Make subdirs as needed
	for _, p := range subdir {
		checkPath(path.Join(api.Htdocs, api.RepositoryPath, p))
	}
	total := len(ids)
	i := 0
	for _, uri := range ids {
		record, err := api.Get(uri)
		if err != nil {
			return err
		}
		basepath := path.Join(api.Htdocs, api.RepositoryPath, subdir[i%q])
		err = api.RenderEPrint(basepath, record)
		if err != nil {
			return err
		}
		//NOTE: We only save the path relative to the web docroot.
		keys = append(keys, path.Join(api.RepositoryPath, subdir[i%q], fmt.Sprintf("%d.json", record.ID)))
		if (i % 1000) == 0 {
			log.Printf("%d of %d records written", i, total)
		}
		i++
	}
	log.Printf("%d of %d records written", i, total)
	src, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(api.Htdocs, api.RepositoryPath, "eprints.json"), src, 0664)
}

// BuildSite generates a website based on the contents of the exported EPrints data.
// The site builder needs to know the name of the BoltDB, the root directory
// for the website and directory to find the templates
func (api *EPrintsAPI) BuildSite(feedSize int, buildEPrintMirror bool) error {
	var err error

	if feedSize < 1 {
		feedSize = DefaultFeedSize
	}

	if buildEPrintMirror == true {
		// Build mirror of repository content.
		log.Printf("Mirroring eprint records")
		err = api.BuildEPrintMirror()
		if err != nil {
			return nil
		}

		/*
			// Build a master file of all records (these are large and probably only useful for migration purposes)
			log.Printf("Building EPrint Repository Master Index")
			err = api.BuildPages(feedSize, "Repository Master Index", path.Join(api.RepositoryPath, "index"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
				return api.GetAllRecords(Descending)
			})
			if err != nil {
				return err
			}
		*/
	}

	// Collect the recent publications (all types)
	log.Printf("Building Recently Published")
	err = api.BuildPages(feedSize, "Recently Published", path.Join("recent", "publications"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
		return api.GetPublications(0, feedSize, Descending)
	})
	if err != nil {
		return err
	}

	// Collect the rencently published  articles
	log.Printf("Building Recent Articles")
	err = api.BuildPages(feedSize, "Recent Articles", path.Join("recent", "articles"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
		return api.GetArticles(start, count, Descending)
	})
	if err != nil {
		return err
	}

	// Collect EPrints by orcid ID and publish
	log.Printf("Building Person (orcid) works")
	orcids, err := api.GetORCIDs(0, -1, Ascending)
	if err != nil {
		return err
	}
	log.Printf("Found %d orcids", len(orcids))
	for _, orcid := range orcids {
		// Build a list of recent ORCID Publications
		err = api.BuildPages(-1, fmt.Sprintf("ORCID: %s", orcid), path.Join("person", fmt.Sprintf("%s", orcid), "recent", "publications"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
			return api.GetORCIDPublications(orcid, start, count, Descending)
		})
		if err != nil {
			return err
		}
		// Build complete list for each orcid
		err = api.BuildPages(-1, fmt.Sprintf("ORCID: %s", orcid), path.Join("person", fmt.Sprintf("%s", orcid), "publications"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
			return api.GetORCIDPublications(orcid, 0, -1, Descending)
		})
		if err != nil {
			return err
		}
		// Build a list of recent ORCID Articles
		err = api.BuildPages(-1, fmt.Sprintf("ORCID: %s", orcid), path.Join("person", fmt.Sprintf("%s", orcid), "recent", "articles"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
			return api.GetORCIDArticles(orcid, start, count, Descending)
		})
		if err != nil {
			return err
		}
		// Build complete list of articels for each ORCID
		err = api.BuildPages(-1, fmt.Sprintf("ORCID: %s", orcid), path.Join("person", fmt.Sprintf("%s", orcid), "articles"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
			return api.GetORCIDArticles(orcid, 0, -1, Descending)
		})
		if err != nil {
			return err
		}
	}

	// Collect EPrints by Group/Affiliation
	log.Printf("Building Local Groups")
	groupNames, err := api.GetLocalGroups(0, -1, Ascending)
	if err != nil {
		return err
	}
	log.Printf("Found %d groups", len(groupNames))
	for _, groupName := range groupNames {
		// Build recently for each affiliation
		err = api.BuildPages(-1, fmt.Sprintf("%s", groupName), path.Join("affiliation", fmt.Sprintf("%s", Slugify(groupName)), "recent", "publications"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
			return api.GetLocalGroupPublications(groupName, start, count, Descending)
		})
		if err != nil {
			return err
		}
		// Build complete list for each affiliation
		err = api.BuildPages(-1, fmt.Sprintf("%s", groupName), path.Join("affiliation", fmt.Sprintf("%s", Slugify(groupName)), "publications"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
			return api.GetLocalGroupPublications(groupName, 0, -1, Descending)
		})
		if err != nil {
			return err
		}
		// Build recent articles for each affiliation
		err = api.BuildPages(-1, fmt.Sprintf("%s", groupName), path.Join("affiliation", fmt.Sprintf("%s", Slugify(groupName)), "recent", "articles"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
			return api.GetLocalGroupArticles(groupName, start, count, Descending)
		})
		if err != nil {
			return err
		}
		// Build complete list of articles for each affiliation
		err = api.BuildPages(-1, fmt.Sprintf("%s", groupName), path.Join("affiliation", fmt.Sprintf("%s", Slugify(groupName)), "articles"), func(api *EPrintsAPI, start, count, direction int) ([]*Record, error) {
			return api.GetLocalGroupArticles(groupName, 0, -1, Descending)
		})
		if err != nil {
			return err
		}
	}
	return nil
}