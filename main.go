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

	lpt "gopkg.in/GeertJohan/go.leptonica.v1"
	gts "gopkg.in/GeertJohan/go.tesseract.v1"
)

var tessData = flag.String("tess_data_prefix", defaultTessData(), "Location of the tesseract data.")
var help = flag.Bool("help", false, "Show this help.")
var pdfDensity = flag.Int("pdfdensity", 300, "density to use when converting pdf's to tiffs.")
var indexLocation = flag.String("index_location", "index.bleve", "Location for the bleve index.")
var hashLocation = flag.String("hash_location", ".indexed_files", "Location where the indexed file hashes are stored.")
var isQuery = flag.Bool("query", false, "Run a query instead of indexing")
var isIndex = flag.Bool("index", false, "Run an indexing operation instead of querying")

func init() {
	// Ensure that org-mode is registered as a mime type.
	mime.AddExtensionType(".org", "text/x-org")
	mime.AddExtensionType(".org_archive", "text/x-org")
}

func defaultTessData() (possible string) {
	possible = os.Getenv("TESSDATA_PREFIX")
	if possible == "" {
		possible = "/usr/local/share"
	}
	return
}

func getPixImage(f string) (*lpt.Pix, error) {
	//log.Print("extension: ", filepath.Ext(f))
	if filepath.Ext(f) == ".pdf" {
		if cmdName, err := exec.LookPath("convert"); err == nil {
			tmpFName := filepath.Join(os.TempDir(), filepath.Base(f)+".tif")
			log.Printf("converting %q to %q", f, tmpFName)
			cmd := exec.Command(cmdName, "-density", fmt.Sprint(*pdfDensity), f, "-depth", "8", tmpFName)
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

// TODO(jwall): Implement the Classifier interface.
type FileData struct {
	FullPath  string
	FileName  string
	MimeType  string
	IndexTime time.Time
	Text      string
}

func (fd *FileData) Type() string {
	return fd.MimeType
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
		MimeType: mt,
		FileName: filepath.Base(file),
		FullPath: path.Clean(file),
		// How to index this properly?
		IndexTime: time.Now(),
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
func IndexFile(file string, hashDir string, index Index) {
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
	fd, err := ProcessFile(file)
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

func IndexDirectory(dir string, hashDir string, index Index) {
	log.Printf("Processing directory: %q", dir)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") ||
				path == *indexLocation || path == *hashLocation {
				return filepath.SkipDir
			}
			return nil
		}
		IndexFile(path, hashDir, index)
		return nil
	})
}

func main() {
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if !(*isQuery) || !(*isIndex) {
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
		for _, file := range flag.Args() {
			fi, err := os.Stat(file)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				log.Printf("Error Stat(ing) file %q", err)
			}
			if fi.IsDir() {
				IndexDirectory(file, *hashLocation, index)
			} else {
				IndexFile(file, *hashLocation, index)
			}
		}
	}
}
