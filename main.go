// Copyright 2015 Jeremy Wall (jeremy@marzhillstudios.com)
// Use of this source code is governed by the Artistic License 2.0.
// That License is included in the LICENSE file.
package main

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// IndexFile indexes a single file using the provided FileProcessor
func IndexFile(file string, p FileProcessor) {
	log.Printf("Processing file: %q", file)
	err := p.Process(file)
	if err != nil {
		log.Printf("Error Processing file %q, %v\n", file, err)
		return
	}
	return
}

// IndexFile indexes all the files in a directory recursively using
// the provided FileProcessor. It skips the directories it uses for storage.
func IndexDirectory(dir string, p FileProcessor) {
	log.Printf("Processing directory: %q", dir)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") ||
				path == *indexLocation || path == *hashLocation {
				return filepath.SkipDir
			}
			return nil
		}
		IndexFile(path, p)
		return nil
	})
}

func main() {
	flag.Parse()

	if *help {
		fmt.Println("goindexer [args] <location to index|search query>")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if !(*isQuery) && !(*isIndex) {
		fmt.Println("One of --query or --index must be passed")
		flag.PrintDefaults()
		os.Exit(1)
	}

	for k, v := range mimeTypeMappings {
		log.Printf("Adding mime-type mapping for extension %q=%q", k, v)
		mime.AddExtensionType(k, v)
	}

	index, err := NewIndex(*indexLocation)
	if err != nil {
		log.Fatalln(err)
	}
	defer index.Close()

	if *isQuery {
		result, err := index.Query(flag.Args())
		if err != nil {
			log.Printf("Error: %q", err)
			os.Exit(1)
		}
		fmt.Println(result)
		return
	} else if *isIndex {
		p := NewProcessor(*hashLocation, index, *force)
		for _, file := range flag.Args() {
			fi, err := os.Stat(file)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				log.Printf("Error Stat(ing) file %q", err)
			}
			if fi.IsDir() {
				IndexDirectory(file, p)
			} else {
				IndexFile(file, p)
			}
		}
	}
}
