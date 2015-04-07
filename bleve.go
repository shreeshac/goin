package main

import (
	"fmt"
	"log"
	"os"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/analysis/analyzers/standard_analyzer"
	"github.com/blevesearch/bleve/analysis/char_filters/html_char_filter"
	"github.com/blevesearch/bleve/registry"
)

const htmlMimeType = "text/html"

func BuildHtmlDocumentMapping() *bleve.DocumentMapping {
	dm := bleve.NewDocumentMapping()
	dm.DefaultAnalyzer = htmlMimeType
	return dm
}

func GetIndex(indexFile string) (bleve.Index, error) {
	// TODO(jwall): An abstract indexing interface?
	var index bleve.Index
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		mapping := bleve.NewIndexMapping()
		mapping.AddDocumentMapping(htmlMimeType, BuildHtmlDocumentMapping())
		// TODO(jwall): Create document mappings for our custom types.
		log.Printf("Creating new index %q", indexFile)
		if index, err = bleve.New(indexFile, mapping); err != nil {
			return nil, fmt.Errorf("Error creating index %q\n", err)
		}
	} else {
		log.Printf("Opening index %q", indexFile)
		if index, err = bleve.Open(indexFile); err != nil {
			return nil, fmt.Errorf("Error opening index %q\n", err)
		}
	}
	return index, nil
}

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
