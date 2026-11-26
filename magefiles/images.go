//go:build mage

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/magefile/mage/sh"
)

// The buckets the card images go to. The public bucket serves the WebP files
// that the page links. The private bucket keeps the PNG originals, so the
// images can be encoded again without a new download.
const (
	imagesPublicURL    = "gs://mklv-keyforge-cards-public/images/cards"
	imagesOriginalsURL = "gs://mklv-keyforge-cards/originals/cards"
	imagesCacheControl = "public, max-age=31536000, immutable"
	imagesWebPQuality  = "85"
)

// Images converts card images to WebP and uploads them. It reads the PNG tree
// that vex's `import-provenance -images=<dir>` writes (<set>/<file>.png),
// writes <set>/<file>.webp at quality 85 next to each PNG, and skips a WebP
// that already exists. It then uploads the WebP files to the public bucket
// with a one-year immutable Cache-Control, and the PNG originals to the
// private bucket. A path never changes content, so a changed image needs a new
// name. Needs cwebp and gcloud. Pass -dir=<images dir>.
func Images(dir string) error {
	pngs, err := findPNGs(dir)
	if err != nil {
		return err
	}

	converted, err := convertToWebP(pngs)
	if err != nil {
		return err
	}

	fmt.Printf("%d PNGs, %d newly converted to WebP\n", len(pngs), converted)

	if err := sh.RunV(
		"gcloud", "storage", "rsync", "--recursive",
		"--exclude=.*\\.(png|tmp)$",
		"--cache-control="+imagesCacheControl,
		dir, imagesPublicURL,
	); err != nil {
		return fmt.Errorf("upload WebP images: %w", err)
	}

	if err := sh.RunV(
		"gcloud", "storage", "rsync", "--recursive",
		"--exclude=.*\\.(webp|tmp)$",
		dir, imagesOriginalsURL,
	); err != nil {
		return fmt.Errorf("upload PNG originals: %w", err)
	}

	return nil
}

// findPNGs lists every .png file under dir, skipping temp files.
func findPNGs(dir string) ([]string, error) {
	var pngs []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".png") {
			pngs = append(pngs, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list images in %s: %w", dir, err)
	}

	if len(pngs) == 0 {
		return nil, fmt.Errorf("no PNG images in %s", dir)
	}

	return pngs, nil
}

// convertToWebP encodes each PNG that has no WebP yet, one cwebp per CPU. It
// writes to a temp file and renames it, so an interrupted run never leaves a
// partial WebP that a rerun would skip.
func convertToWebP(pngs []string) (int, error) {
	jobs := make(chan string)

	var (
		mu        sync.Mutex
		converted int
		errs      []error
		wg        sync.WaitGroup
	)

	for range runtime.NumCPU() {
		wg.Go(func() {
			for png := range jobs {
				done, err := convertOne(png)

				mu.Lock()
				if err != nil {
					errs = append(errs, err)
				} else if done {
					converted++
				}
				mu.Unlock()
			}
		})
	}

	for _, png := range pngs {
		jobs <- png
	}

	close(jobs)
	wg.Wait()

	return converted, errors.Join(errs...)
}

// convertOne encodes one PNG. It reports false when the WebP already exists.
func convertOne(png string) (bool, error) {
	webp := strings.TrimSuffix(png, ".png") + ".webp"

	if info, err := os.Stat(webp); err == nil && info.Size() > 0 {
		return false, nil
	}

	tmp := webp + ".tmp"

	if err := sh.Run(
		"cwebp",
		"-quiet",
		"-q",
		imagesWebPQuality,
		"-m",
		"6",
		png,
		"-o",
		tmp,
	); err != nil {
		_ = os.Remove(tmp)

		return false, fmt.Errorf("convert %s: %w", png, err)
	}

	if err := os.Rename(tmp, webp); err != nil {
		return false, fmt.Errorf("rename %s: %w", tmp, err)
	}

	return true, nil
}
