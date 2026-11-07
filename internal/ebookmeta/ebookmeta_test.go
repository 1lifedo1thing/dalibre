package ebookmeta

import (
	"archive/zip"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

var jpegBytes = []byte{0xff, 0xd8, 0xff, 0xe0, 1, 2, 3}

func writeEPUB(t *testing.T, opf string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Some File - Someone.epub")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	add := func(name string, data []byte) {
		w, _ := zw.Create(name)
		w.Write(data)
	}
	add("mimetype", []byte("application/epub+zip"))
	add("META-INF/container.xml", []byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))
	add("OEBPS/content.opf", []byte(opf))
	add("OEBPS/images/cover.jpg", jpegBytes)
	zw.Close()
	f.Close()
	return p
}

func TestEPUB3(t *testing.T) {
	p := writeEPUB(t, `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:title>  The   Three-Body Problem </dc:title>
  <dc:creator id="c1">Liu Cixin</dc:creator>
  <meta refines="#c1" property="role">aut</meta>
  <dc:creator id="c2">Ken Liu</dc:creator>
  <meta refines="#c2" property="role">trl</meta>
  <dc:language>en</dc:language>
  <dc:publisher>Tor Books</dc:publisher>
  <dc:date>2014-11-11</dc:date>
  <dc:identifier>urn:isbn:978-0-7653-8203-0</dc:identifier>
  <dc:subject>Science Fiction</dc:subject>
  <meta name="calibre:series" content="Remembrance of Earth's Past"/>
  <meta name="calibre:series_index" content="1"/>
</metadata>
<manifest><item id="img" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/></manifest>
</package>`)
	m, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "The Three-Body Problem" || len(m.Authors) != 1 || m.Authors[0] != "Liu Cixin" {
		t.Errorf("title/authors: %q %v", m.Title, m.Authors)
	}
	if m.ISBN != "9780765382030" || m.Publisher != "Tor Books" || m.Language != "en" || m.Pubdate.Year() != 2014 {
		t.Errorf("meta: %+v", m)
	}
	if m.Series != "Remembrance of Earth's Past" || m.SeriesIndex != 1 || len(m.Tags) != 1 {
		t.Errorf("series/tags: %+v", m)
	}
	if string(m.Cover) != string(jpegBytes) {
		t.Errorf("cover not found")
	}
}

func TestEPUB2CoverMetaAndFallbackName(t *testing.T) {
	p := writeEPUB(t, `<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
  <meta name="cover" content="cov"/>
</metadata>
<manifest><item id="cov" href="images/cover.jpg" media-type="image/jpeg"/></manifest>
</package>`)
	m, _ := Read(p)
	if m.Title != "Some File" || len(m.Authors) != 1 || m.Authors[0] != "Someone" {
		t.Errorf("fallback to file name: %q %v", m.Title, m.Authors)
	}
	if len(m.Cover) == 0 {
		t.Errorf("EPUB 2 cover meta not followed")
	}
}

// buildMOBI writes a minimal PalmDB with a MOBI header, EXTH and an image record.
func buildMOBI(t *testing.T) string {
	t.Helper()
	be := binary.BigEndian
	name := []byte("Foundation")
	var exth []byte
	rec := func(typ uint32, val []byte) {
		h := make([]byte, 8)
		be.PutUint32(h, typ)
		be.PutUint32(h[4:], uint32(8+len(val)))
		exth = append(exth, append(h, val...)...)
	}
	rec(100, []byte("Isaac Asimov"))
	rec(503, []byte("Foundation (Updated)"))
	off := make([]byte, 4)
	rec(201, off) // cover offset 0
	exthHead := make([]byte, 12)
	copy(exthHead, "EXTH")
	be.PutUint32(exthHead[4:], uint32(12+len(exth)))
	be.PutUint32(exthHead[8:], 3)

	const mobiHeaderLen = 232
	r0 := make([]byte, 16+mobiHeaderLen)
	copy(r0[16:], "MOBI")
	be.PutUint32(r0[20:], mobiHeaderLen)
	be.PutUint32(r0[28:], 65001)
	be.PutUint32(r0[108:], 1) // first image record
	be.PutUint32(r0[128:], 0x40)
	r0 = append(r0, exthHead...)
	r0 = append(r0, exth...)
	be.PutUint32(r0[84:], uint32(len(r0)))
	be.PutUint32(r0[88:], uint32(len(name)))
	r0 = append(r0, name...)

	records := [][]byte{r0, jpegBytes}
	header := make([]byte, 78+8*len(records)+2)
	copy(header[60:], "BOOKMOBI")
	be.PutUint16(header[76:], uint16(len(records)))
	pos := len(header)
	for i, r := range records {
		be.PutUint32(header[78+8*i:], uint32(pos))
		pos += len(r)
	}
	out := header
	for _, r := range records {
		out = append(out, r...)
	}
	p := filepath.Join(t.TempDir(), "book.mobi")
	os.WriteFile(p, out, 0o644)
	return p
}

func TestMOBI(t *testing.T) {
	m, err := Read(buildMOBI(t))
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Foundation (Updated)" || len(m.Authors) != 1 || m.Authors[0] != "Isaac Asimov" {
		t.Errorf("mobi meta: %q %v", m.Title, m.Authors)
	}
	if string(m.Cover) != string(jpegBytes) {
		t.Errorf("mobi cover: %v", m.Cover)
	}
}

func TestPDFUsesFileName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Thinking, Fast and Slow - Daniel Kahneman.pdf")
	os.WriteFile(p, []byte("%PDF-1.4"), 0o644)
	m, _ := Read(p)
	if m.Title != "Thinking, Fast and Slow" || m.Authors[0] != "Daniel Kahneman" {
		t.Errorf("pdf: %+v", m)
	}
}
