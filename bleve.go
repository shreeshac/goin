// Copyright 2015 Jeremy Wall (jeremy@marzhillstudios.com)
// Use of this source code is governed by the Artistic License 2.0.
// That License is included in the LICENSE file.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/analysis/char_filters/html_char_filter"
	"github.com/blevesearch/bleve/analysis/language/en"
	"github.com/blevesearch/bleve/registry"
)

const htmlMimeType = "text/html"

// handle text/html types
func init() {
	registry.RegisterAnalyzer(htmlMimeType, func(config map[string]interface{}, cache *registry.Cache) (*analysis.Analyzer, error) {
		a, err := en.AnalyzerConstructor(config, cache)
		if err != nil {
			if cf, err := cache.CharFilterNamed(html_char_filter.Name); err == nil {
				a.CharFilters = []analysis.CharFilter{cf}
			} else {
				return nil, err
			}
		}

		return a, err
	})
}

func buildHtmlDocumentMapping() *bleve.DocumentMapping {
	dm := bleve.NewDocumentMapping()
	dm.DefaultAnalyzer = htmlMimeType
	return dm
}

type Index interface {
	Put(data *FileData) error
	Query(terms []string) (*bleve.SearchResult, error)
	Close() error
}

type bleveIndex struct {
	index bleve.Index
}

func (i *bleveIndex) Put(data *FileData) error {
	if err := i.index.Index(data.FullPath, data); err != nil {
		return fmt.Errorf("Error writing to index: %q", err)
	}
	return nil
}

func (i *bleveIndex) Query(terms []string) (*bleve.SearchResult, error) {
	searchQuery := strings.Join(terms, " ")
	query := bleve.NewQueryStringQuery(searchQuery)
	// TODO(jwall): limit, skip, and explain should be configurable.
	request := bleve.NewSearchRequestOptions(query, *limit, *from, false)
	// TODO(jwall): This should be configurable too.
	request.Highlight = bleve.NewHighlightWithStyle("ansi")

	result, err := i.index.Search(request)
	if err != nil {
		log.Printf("Search Error: %q", err)
		return nil, err
	}
	return result, nil
}

func (i *bleveIndex) Close() error {
	return i.index.Close()
}
func NewIndex(indexLocation string) (Index, error) {
	// TODO(jwall): An abstract indexing interface?
	var index bleve.Index
	if _, err := os.Stat(indexLocation); os.IsNotExist(err) {
		mapping := bleve.NewIndexMapping()
		mapping.DefaultAnalyzer = "en"
		mapping.AddDocumentMapping(htmlMimeType, buildHtmlDocumentMapping())
		// TODO(jwall): Create document mappings for our custom types.
		log.Printf("Creating new index %q", indexLocation)
		if index, err = bleve.New(indexLocation, mapping); err != nil {
			return nil, fmt.Errorf("Error creating index %q\n", err)
		}
	} else {
		log.Printf("Opening index %q", indexLocation)
		if index, err = bleve.Open(indexLocation); err != nil {
			return nil, fmt.Errorf("Error opening index %q\n", err)
		}
	}
	return &bleveIndex{index}, nil
}
