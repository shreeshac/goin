package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/blevesearch/bleve"
	lpt "gopkg.in/GeertJohan/go.leptonica.v1"
	gts "gopkg.in/GeertJohan/go.tesseract.v1"
)

/*
  DevNotes:

  1. We need to detect when we've already scanned a document. Probably using a hash.
  2. We need to implement the full text indexing of the documents.
  3. We need a standard way to query the documents.
  4. Flesh out a way to handle various file types easily.

  Indexing:
    1. Path based off of a root path.
    2. mime-type
    3.
*/

var tessData = flag.String("tess_data_prefix", defaultTessData(), "Location of the tesseract data")
var help = flag.Bool("help", false, "Show this help")
var pdfDensity = flag.Int("pdfdensity", 300, "density to use when converting pdf's to tiffs")
var rootPath = flag.String("root_path", "/", "Root path for indexing")
var indexLocation = flag.String("index_location", "index.bleve", "Location for the bleve index rooted at the rootPath")
var hashLocation = flag.String("hash_location", ".indexed_files", "Location where the indexed file hashes are stored.")

func defaultTessData() (possible string) {
	possible = os.Getenv("TESSDATA_PREFIX")
	if possible == "" {
		possible = "/usr/local/share"
	}
	return
}

func getPixImage(f string) (*lpt.Pix, error) {
	log.Print("extension: ", filepath.Ext(f))
	if filepath.Ext(f) == ".pdf" {
		// TODO(jwall): handle pdfs by converting them first.
		if cmdName, err := exec.LookPath("convert"); err == nil {
			tmpFName := filepath.Join(os.TempDir(), filepath.Base(f)+".tif")
			log.Printf("converting %q to %q", f, tmpFName)
			cmd := exec.Command(cmdName, "-density", "300", f, "-depth", "8", tmpFName)
			out, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("output: %q", out)
				return nil, fmt.Errorf("Error converting pdf with %q err: %v", cmd.Args, err)
			}
			f = tmpFName
		} else {
			return nil, fmt.Errorf("Unable to find convert binary %v", err)
		}
	}
	log.Printf("getting pix from %q", f)
	return lpt.NewPixFromFile(f)
}

type FileData struct {
	RelativePath string
	FullPath     string
	FileName     string
	MimeType     string
	IndexTime    time.Time
	Hash         []byte
	Text         string
}

func ocrImageFile(file string) (string, error) {
	// Create new tess instance and point it to the tessdata location.
	// Set language to english.
	t, err := gts.NewTess(filepath.Join(*tessData, "tessdata"), "eng")
	if err != nil {
		log.Fatalf("Error while initializing Tess: %s\n", err)
	}
	defer t.Close()

	pix, err := getPixImage(file)
	if err != nil {
		return "", fmt.Errorf("Error while getting pix from file: %s (%s)", file, err)
	}
	defer pix.Close()

	t.SetPageSegMode(gts.PSM_AUTO_OSD)

	// TODO(jwall): What is this even?
	err = t.SetVariable("tessedit_char_whitelist", ` !"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\]^_abcdefghijklmnopqrstuvwxyz{|}~`+"`")
	if err != nil {
		return "", fmt.Errorf("Failed to set variable: %s\n", err)
	}

	t.SetImagePix(pix)

	return t.Text(), nil
}

func getPlainTextContent(file string) (string, error) {
	fd, err := os.Open(file)
	defer fd.Close()
	if err != nil {
		return "", err
	}
	bs, err := ioutil.ReadAll(fd)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func ProcessFile(file string) (*FileData, error) {
	// TODO(jeremy): Use hashing to detect if we've already indexed this file.
	ext := filepath.Ext(file)
	// TODO(jwall): Do I want to do anything with the params?
	mt, _, err := mime.ParseMediaType(mime.TypeByExtension(ext))
	parts := strings.SplitN(mt, "/", 2)
	if err != nil {
		return nil, fmt.Errorf("Error hashing file %q", err)
	}
	fd := FileData{
		MimeType:     mt,
		FileName:     filepath.Base(file),
		FullPath:     path.Clean(file),
		RelativePath: strings.Replace(filepath.Dir(file), path.Clean(*rootPath)+"/", "", 1),
		IndexTime:    time.Now(),
	}
	//log.Printf("Detected mime category: %q", parts[0])
	// TODO(jeremy): We need an abstract file type handler interface and
	// a way to register them.
	switch parts[0] {
	case "text":
		fd.Text, err = getPlainTextContent(file)
		if err != nil {
			return nil, err
		}
	case "application":
		if strings.ToLower(ext) == ".pdf" {
			fd.Text, err = ocrImageFile(file)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("Unhandled file format %q", mt)
		}
	case "image":
		fd.Text, err = ocrImageFile(file)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unhandled file format %q", mt)
	}
	return &fd, nil
}

func GetIndex(indexFile string) (bleve.Index, error) {
	// TODO(jwall): An abstract indexing interface?
	var index bleve.Index
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		mapping := bleve.NewIndexMapping()
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

func main() {
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	indexFile := path.Join(*rootPath, *indexLocation)
	hashDir := path.Join(*rootPath, *hashLocation)
	index, err := GetIndex(indexFile)
	if err != nil {
		log.Fatalln(err)
	}
	defer index.Close()
	for _, file := range flag.Args() {
		h, err := HashFile(file)
		if ok, _ := CheckHash(filepath.Base(file), h, hashDir); ok {
			log.Printf("Already indexed %q", file)
			continue
		}
		// TODO(jwall): Check the hash against the last processed index.
		fd, err := ProcessFile(file)
		if err != nil {
			log.Printf("Error reading file %q, %v\n", file, err)
		}
		log.Printf("Indexing %q", fd.FullPath)
		if err := index.Index(fd.FileName, fd); err != nil {
			log.Printf("Error writing to index: %q", err)
		}
		if err := WriteFileHash(fd.FileName, h, hashDir); err != nil {
			log.Printf("Error writing file hash %q", err)
		}
	}
}
