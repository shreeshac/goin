package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	lpt "gopkg.in/GeertJohan/go.leptonica.v1"
	gts "gopkg.in/GeertJohan/go.tesseract.v1"

	// TODO(jwall): import "github.com/blevesearch/bleve"
)

/*
  DevNotes:

  1. We need to detect when we've already scanned a document. Probably using a hash.
  2. We need to implement the full text indexing of the documents.
  3. We need a standard way to query the documents.
  4. Flesh out a way to handle various file types easily.
*/

func defaultTessData() (possible string) {
	possible = os.Getenv("TESSDATA_PREFIX")
	if possible == "" {
		possible = "/usr/local/share"
	}
	return
}

var tessData = flag.String("tess_data_prefix", defaultTessData(), "Location of the tesseract data")
var help = flag.Bool("help", false, "Show this help")
var pdfDensity = flag.Int("pdfdensity", 300, "density to use when converting pdf's to tiffs")

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

func main() {
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}
	// Create new tess instance and point it to the tessdata location.
	// Set language to english.
	t, err := gts.NewTess(filepath.Join(*tessData, "tessdata"), "eng")
	if err != nil {
		log.Fatalf("Error while initializing Tess: %s\n", err)
	}
	defer t.Close()

	for _, file := range flag.Args() {
		// TODO(jwall): Handle pdf
		pix, err := getPixImage(file)
		if err != nil {
			log.Fatalf("Error while getting pix from file: %s (%s)", file, err)
		}
		defer pix.Close()

		t.SetPageSegMode(gts.PSM_AUTO_OSD)

		// TODO(jwall): What is this even?
		err = t.SetVariable("tessedit_char_whitelist", ` !"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\]^_abcdefghijklmnopqrstuvwxyz{|}~`+"`")
		if err != nil {
			log.Fatalf("Failed to set variable: %s\n", err)
		}

		t.SetImagePix(pix)

		fmt.Println(t.Text())
	}
}
