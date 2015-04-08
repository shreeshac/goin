package main

import (
	"fmt"
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

type FileTranslator func(string) (string, error)

// TODO(jwall): Okay large file support without having to load the entire file
// into memory would be nice.
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

type FileProcessor interface {
	Process(file string) (*FileData, error)
	Register(mime string, ft FileTranslator) error
}

type processor struct {
	defaultMimeTypeHandlers map[string]FileTranslator
}

func (p *processor) registerDefaults() {
	p.defaultMimeTypeHandlers = map[string]FileTranslator{
		"text":                   getPlainTextContent,
		"image":                  ocrImageFile,
		"application/javascript": getPlainTextContent,
		// TODO(jeremy): We should try the pdf2text application first if
		// available.
		"application/pdf": ocrImageFile,
	}

}

func NewProcessor() FileProcessor {
	p := &processor{}
	p.registerDefaults()
	return p
}

func (p *processor) Register(mime string, ft FileTranslator) error {
	if _, exists := p.defaultMimeTypeHandlers[mime]; exists {
		return fmt.Errorf("Attempt to register already existing mime type FileTranslator %q", mime)
	}
	p.defaultMimeTypeHandlers[mime] = ft
	return nil
}

func (p *processor) Process(file string) (*FileData, error) {
	// TODO(jeremy): Move the hashing part out of here.
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
	log.Printf("Detected mime category: %q", parts[0])
	if ft, exists := p.defaultMimeTypeHandlers[mt]; exists {
		fd.Text, err = ft(file)
		if err != nil {
			return nil, err
		}
	} else if ft, exists := p.defaultMimeTypeHandlers[parts[0]]; exists {
		fd.Text, err = ft(file)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("Unhandled file format %q", mt)
	}
	return &fd, nil
}
