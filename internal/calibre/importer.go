package calibre

import (
	"strings"

	"dalibre/internal/ebookmeta"
)

// iso639_2 maps two-letter codes to the three-letter ones calibre stores.
var iso639_2 = map[string]string{
	"en": "eng", "zh": "zho", "ja": "jpn", "ko": "kor", "fr": "fra", "de": "deu", "es": "spa", "it": "ita",
	"pt": "por", "ru": "rus", "ar": "ara", "nl": "nld", "sv": "swe", "no": "nor", "nb": "nob", "da": "dan",
	"fi": "fin", "pl": "pol", "cs": "ces", "tr": "tur", "el": "ell", "he": "heb", "hi": "hin", "vi": "vie",
	"th": "tha", "id": "ind", "uk": "ukr", "hu": "hun", "ro": "ron", "la": "lat",
}

// LanguageCode turns "en", "en-US" or "eng" into calibre's "eng".
func LanguageCode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "und" {
		return ""
	}
	base, _, _ := strings.Cut(strings.ReplaceAll(s, "_", "-"), "-")
	if c, ok := iso639_2[base]; ok {
		return c
	}
	if len(base) == 3 {
		return base
	}
	return ""
}

// Import adds an e-book file as a new book, reading its metadata and
// cover from the file.
func (l *Library) Import(file string) (int64, error) {
	m, _ := ebookmeta.Read(file) // a file we can't parse is still added, named after itself
	md := Metadata{
		Title: m.Title, Authors: m.Authors, Publisher: m.Publisher, Tags: m.Tags,
		Pubdate: m.Pubdate, Series: m.Series, SeriesIndex: m.SeriesIndex, Comments: m.Description,
		Identifiers: map[string]string{},
	}
	if m.ISBN != "" {
		md.Identifiers["isbn"] = m.ISBN
	}
	if code := LanguageCode(m.Language); code != "" {
		md.Languages = []string{code}
	}
	return l.AddBook(NewBook{Metadata: md, File: file, Cover: m.Cover})
}
