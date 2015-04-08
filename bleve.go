package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/analysis/analyzers/standard_analyzer"
	"github.com/blevesearch/bleve/analysis/char_filters/html_char_filter"
	"github.com/blevesearch/bleve/registry"
)

const htmlMimeType = "text/html"

// handle text/html types
func init() {
	registry.RegisterAnalyzer(htmlMimeType, func(config map[string]interface{}, cache *registry.Cache) (*analysis.Analyzer, error) {
		a, err := standard_analyzer.AnalyzerConstructor(config, cache)
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
	Index(data *FileData) error
	Query(terms []string) (string, error)
	Close() error
}

type bleveIndex struct {
	index bleve.Index
}

func (i *bleveIndex) Index(data *FileData) error {
	if err := i.index.Index(data.FileName, data); err != nil {
		return fmt.Errorf("Error writing to index: %q", err)
	}
	return nil
}

func (i *bleveIndex) Query(terms []string) (string, error) {
	searchQuery := strings.Join(terms, " ")
	query := bleve.NewQueryStringQuery(searchQuery)
	// TODO(jwall): limit, skip, and explain should be configurable.
	request := bleve.NewSearchRequestOptions(query, 10, 0, false)
	// TODO(jwall): This should be configurable too.
	request.Highlight = bleve.NewHighlightWithStyle("ansi")

	result, err := i.index.Search(request)
	if err != nil {
		log.Printf("Search Error: %q", err)
		return "", err
	}
	return fmt.Sprintln(result), nil
}

func (i *bleveIndex) Close() error {
	return i.index.Close()
}
func NewIndex(indexLocation string) (Index, error) {
	// TODO(jwall): An abstract indexing interface?
	var index bleve.Index
	if _, err := os.Stat(indexLocation); os.IsNotExist(err) {
		mapping := bleve.NewIndexMapping()
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
