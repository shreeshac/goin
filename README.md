Goin Full Text Search for your files
====================================

Goin is a full text search indexer using
https://github.com/blevesearch/bleve for your files on disk. It can
handle plain text, many different images, as well as pdf files if the
correct utilities are installed.

It processes files based on their mime type making it fairly easy to
add support for more files in the future. It's still very much a work
in progress.

Usage
=====

goin --help will give you an overview of the flags to tweak it's operation.

Install
=======

`go get bitbucket.org/zaphar/goin` will install the tool.

For pdf support goin needs a few extra items. The ImageMagick convert tool as well as the xpdf suite of tools. Goin uses these to first try to get text out of the pdf and falling back to OCR if the pdf is only images.
