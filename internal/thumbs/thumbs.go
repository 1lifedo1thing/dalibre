// Package thumbs makes small copies of cover images, kept on disk, so a
// grid of hundreds of covers decodes kilobytes instead of megabytes.
package thumbs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

// Cache stores thumbnails in a folder.
type Cache struct{ Dir string }

// New uses the user's cache folder.
func New() (*Cache, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, "Dalibre", "thumbs")
	return &Cache{Dir: dir}, os.MkdirAll(dir, 0o755)
}

// Get returns the path of a thumbnail of src at most height pixels tall,
// making it when src is newer than the one kept.
func (c *Cache) Get(src string, height int) (string, error) {
	st, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(fmt.Appendf(nil, "%s|%d|%d|%d", src, st.ModTime().UnixNano(), st.Size(), height))
	dst := filepath.Join(c.Dir, hex.EncodeToString(sum[:10])+".jpg")
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	if b.Dy() > height {
		w := max(1, b.Dx()*height/b.Dy())
		small := image.NewRGBA(image.Rect(0, 0, w, height))
		draw.CatmullRom.Scale(small, small.Bounds(), img, b, draw.Src, nil)
		img = small
	}
	tmp, err := os.CreateTemp(c.Dir, "thumb-*")
	if err != nil {
		return "", err
	}
	if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: 88}); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	if err := os.Rename(tmp.Name(), dst); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return dst, nil
}
