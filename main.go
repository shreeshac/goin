package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var tessData = flag.String("tess_data_prefix", defaultTessData(), "Location of the tesseract data.")
var help = flag.Bool("help", false, "Show this help.")
var pdfDensity = flag.Int("pdfdensity", 300, "density to use when converting pdf's to tiffs.")
var indexLocation = flag.String("index_location", "index.bleve", "Location for the bleve index.")
var hashLocation = flag.String("hash_location", ".indexed_files", "Location where the indexed file hashes are stored.")
var isQuery = flag.Bool("query", false, "Run a query instead of indexing")
var isIndex = flag.Bool("index", false, "Run an indexing operation instead of querying")

// Hash optimizations
func HashFile(file string) ([]byte, error) {
	h := sha256.New()
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(h, f)
	if err != nil {
		return nil, err
	}
	return h.Sum([]byte{}), nil
}

func CheckHash(file string, hash []byte, hashDir string) (bool, error) {
	hashFile := path.Join(hashDir, file)
	if _, err := os.Stat(hashFile); os.IsNotExist(err) {
		return false, nil
	}
	f, err := os.Open(hashFile)
	defer f.Close()
	if err != nil {
		return false, err
	}
	bs, err := ioutil.ReadAll(f)
	if err != nil {
		return false, err
	}
	if len(bs) != len(hash) {
		return false, nil
	}
	for i, b := range bs {
		if b != hash[i] {
			return false, nil
		}
	}
	return true, nil
}

func WriteFileHash(file string, hash []byte, hashDir string) error {
	if _, err := os.Stat(hashDir); os.IsNotExist(err) {
		if err := os.MkdirAll(hashDir, os.ModeDir|os.ModePerm); err != nil {
			return err
		}
	}
	fd, err := os.Create(filepath.Join(hashDir, file))
	defer fd.Close()
	if err != nil {
		return err
	}
	_, err = fd.Write(hash)
	return err
}

// Entry points
func IndexFile(file string, hashDir string, p FileProcessor, index Index) {
	log.Printf("Processing file: %q", file)
	fi, err := os.Stat(file)
	if fi.Size() > 1000000 {
		log.Printf("File too large to index %q", file)
		return
	}

	h, err := HashFile(file)
	if ok, _ := CheckHash(filepath.Base(file), h, hashDir); ok {
		log.Printf("Already indexed %q", file)
		return
	}
	fd, err := p.Process(file)
	if err != nil {
		log.Printf("Error Processing file %q, %v\n", file, err)
		return
	}
	log.Printf("Indexing %q", fd.FullPath)
	if err := index.Index(fd); err != nil {
		log.Printf("Error writing to index: %q", err)
		return
	}
	if err := WriteFileHash(fd.FileName, h, hashDir); err != nil {
		log.Printf("Error writing file hash %q", err)
		return
	}
	return
}

func IndexDirectory(dir string, hashDir string, p FileProcessor, index Index) {
	log.Printf("Processing directory: %q", dir)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") ||
				path == *indexLocation || path == *hashLocation {
				return filepath.SkipDir
			}
			return nil
		}
		IndexFile(path, hashDir, p, index)
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
		p := NewProcessor()
		for _, file := range flag.Args() {
			fi, err := os.Stat(file)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				log.Printf("Error Stat(ing) file %q", err)
			}
			if fi.IsDir() {
				IndexDirectory(file, *hashLocation, p, index)
			} else {
				IndexFile(file, *hashLocation, p, index)
			}
		}
	}
}
