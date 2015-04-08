package main

import (
	"flag"
	"fmt"
	"strings"
)

type StringMapFlag map[string]string

func (v StringMapFlag) String() string {
	return fmt.Sprint(map[string]string(v))
}

func (v StringMapFlag) Set(s string) error {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) < 2 {
		return fmt.Errorf("Invalid mimetype mapping")
	}
	v[parts[0]] = parts[1]
	return nil
}

func mimeFlag(name, usage string) StringMapFlag {
	mimeTypeMappings := StringMapFlag{}
	flag.Var(mimeTypeMappings, name, usage)
	return mimeTypeMappings
}

var tessData = flag.String("tess_data_prefix", defaultTessData(), "Location of the tesseract data.")
var help = flag.Bool("help", false, "Show this help.")
var pdfDensity = flag.Int("pdfdensity", 300, "density to use when converting pdf's to tiffs.")
var indexLocation = flag.String("index_location", "index.bleve", "Location for the bleve index.")
var hashLocation = flag.String("hash_location", ".indexed_files", "Location where the indexed file hashes are stored.")
var isQuery = flag.Bool("query", false, "Run a query instead of indexing")
var isIndex = flag.Bool("index", false, "Run an indexing operation instead of querying")
var mimeTypeMappings = mimeFlag("mime", "Add a custom mime type mapping.")
