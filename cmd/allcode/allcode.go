// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cheggaaa/pb/v3"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/sumdb/tlog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

var httpClient = &http.Client{
	Timeout: 60 * time.Minute,
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 1024,
	},
}

func newRequestWithContext(ctx context.Context, method, url string) *http.Request {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Disable-Module-Fetch", "true")
	return req
}

type Index struct {
	last time.Time
	d    *json.Decoder
}

func NewIndex(ctx context.Context) (*Index, error) {
	i := &Index{}
	if err := i.nextPage(ctx); err != nil {
		return nil, err
	}
	return i, nil
}

func (i *Index) nextPage(ctx context.Context) error {
	url := "https://index.golang.org/index?since=" + i.last.Add(1).Format(time.RFC3339Nano)
	req, err := httpClient.Do(newRequestWithContext(ctx, "GET", url))
	if err != nil {
		return err
	}
	i.d = json.NewDecoder(req.Body)
	return nil
}

type Version struct {
	Path, Version string
	Timestamp     time.Time
}

func (i *Index) next(ctx context.Context) (*Version, error) {
	v := &Version{}
	err := i.d.Decode(v)
	if err == io.EOF {
		if err := i.nextPage(ctx); err != nil {
			return nil, err
		}
		err = i.d.Decode(v)
	}
	if err != nil {
		return nil, err
	}
	i.last = v.Timestamp
	return v, nil
}

func fetchLatest(ctx context.Context) ([]byte, error) {
	url := "https://sum.golang.org/latest"
	res, err := httpClient.Do(newRequestWithContext(ctx, "GET", url))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %q: %v", url, res.Status)
	}
	return io.ReadAll(res.Body)
}

var errorInvalidName = errors.New("invalid name")

func proxyURL(path, version, suffix string) (string, error) {
	p, err := module.EscapePath(path)
	if err != nil {
		return "", errorInvalidName
	}
	v, err := module.EscapeVersion(version)
	if err != nil {
		return "", errorInvalidName
	}
	return "https://proxy.golang.org/" + p + "/@v/" + v + suffix, nil
}

var errorGone = errors.New("410 Gone")

func fetchMod(ctx context.Context, path, version string) ([]byte, error) {
	url, err := proxyURL(path, version, ".mod")
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(newRequestWithContext(ctx, "GET", url))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusGone {
		return nil, errorGone
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %q: %v", url, res.Status)
	}
	return io.ReadAll(res.Body)
}

func fetchZipHead(ctx context.Context, path, version string) (string, int64, error) {
	url, err := proxyURL(path, version, ".zip")
	if err != nil {
		return "", 0, err
	}
	res, err := httpClient.Do(newRequestWithContext(ctx, "HEAD", url))
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("GET %q: %v", url, res.Status)
	}
	return res.Request.URL.String(), res.ContentLength, nil
}

func fetchZip(ctx context.Context, path, version string, w io.Writer) error {
	url, err := proxyURL(path, version, ".zip")
	if err != nil {
		return err
	}
	res, err := httpClient.Do(newRequestWithContext(ctx, "GET", url))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusGone {
		return errorGone
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %q: %v", url, res.Status)
	}
	// I experimented with using Range requests to back the zip
	// ReaderAt, but it was extremely slow.
	_, err = io.Copy(w, res.Body)
	return err
}

var pbTemplate pb.ProgressBarTemplate = `{{string . "prefix"}} {{counters . }} {{bar . }} {{percent . }} {{etime . }}`

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to FILE")
	memprofile := flag.String("memprofile", "", "write memory profile to FILE")
	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatal("usage: allcode /mnt/allcode")
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	log.SetFlags(log.Lshortfile | log.Flags())
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	latest, err := fetchLatest(ctx)
	if err != nil {
		log.Fatal(err)
	}
	tree, err := tlog.ParseTree(latest)
	if err != nil {
		log.Fatal(err)
	}

	bar := pbTemplate.Start64(tree.N).Set("prefix", "Fetching index...")
	latestVersions := make(map[string]string)

	i, err := NewIndex(ctx)
	if err != nil {
		log.Fatal(err)
	}

	var linesSeen uint64
	for {
		v, err := i.next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		linesSeen++
		bar.Increment()

		if semver.Compare(v.Version, latestVersions[v.Path]) >= 0 {
			latestVersions[v.Path] = v.Version
		}
	}
	bar.Finish()

	pool := &sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}

	bar = pbTemplate.Start(len(latestVersions)).Set("prefix", "Fetching modules...")
	sem := semaphore.NewWeighted(200)
	gcp := semaphore.NewWeighted(500) // GCS can take it, and it's way way slower
	g, ctx := errgroup.WithContext(ctx)

	var gone, invalidName, vendor, mismatchedGoMod, invalidGoMod int64
	var noGoCode, noGoMod, gcsRedirect, good, goBytes, allBytes, goFilesCount int64
	for path, version := range latestVersions {
		if err := ctx.Err(); ctx.Err() != nil {
			bar.Finish()
			log.Println(err)
			break
		}

		if err := sem.Acquire(ctx, 1); err != nil {
			bar.Finish()
			log.Println(err)
			break
		}

		path, version := path, version
		g.Go(func() error {
			releaseOnce := &sync.Once{}
			defer releaseOnce.Do(func() { sem.Release(1) })
			defer bar.Increment()

			if strings.Contains(path, "/vendor/") || strings.Contains(path, "/kubernetes/staging/") {
				atomic.AddInt64(&vendor, 1)
				return nil
			}
			modBytes, err := fetchMod(ctx, path, version)
			if err == errorInvalidName {
				atomic.AddInt64(&invalidName, 1)
				return nil
			}
			if err == errorGone {
				atomic.AddInt64(&gone, 1)
				return nil
			}
			if err != nil {
				return err
			}
			mod, err := modfile.ParseLax(path+"@"+version, modBytes, nil)
			if err != nil {
				atomic.AddInt64(&invalidGoMod, 1)
				return nil
			}
			if mod.Module.Mod.Path != path {
				atomic.AddInt64(&mismatchedGoMod, 1)
				return nil
			}

			url, size, err := fetchZipHead(ctx, path, version)
			if err != nil {
				return err
			}
			if strings.HasPrefix(url, "https://storage.googleapis.com/") {
				atomic.AddInt64(&gcsRedirect, 1)
				releaseOnce.Do(func() { sem.Release(1) })
				gcp.Acquire(ctx, 1)
				defer gcp.Release(1)
			}

			zipBytes := pool.Get().(*bytes.Buffer)
			defer func() {
				if zipBytes.Len() > 10*1024*1024 {
					zipBytes.Reset()
					pool.Put(zipBytes)
				}
			}()
			zipBytes.Grow(int(size))

			if err := fetchZip(ctx, path, version, zipBytes); err != nil {
				return err
			}
			atomic.AddInt64(&allBytes, size)

			zipBytesReader := bytes.NewReader(zipBytes.Bytes())
			z, err := zip.NewReader(zipBytesReader, size)
			if err != nil {
				return err
			}

			var hasGoMod bool
			goFiles := make([]*zip.File, 0, len(z.File))
			for _, f := range z.File {
				if strings.HasSuffix(f.Name, ".go") {
					goFiles = append(goFiles, f)
				}
				if strings.HasSuffix(f.Name, "/go.mod") {
					hasGoMod = true
				}
			}
			if len(goFiles) == 0 {
				atomic.AddInt64(&noGoCode, 1)
				return nil
			}
			if !hasGoMod {
				atomic.AddInt64(&noGoMod, 1)
				return nil
			}
			atomic.AddInt64(&good, 1)

			for _, zf := range goFiles {
				src, err := z.Open(zf.Name)
				if err != nil {
					return err
				}

				name := filepath.Join(flag.Arg(0), zf.Name)
				if err := os.MkdirAll(filepath.Dir(name), 0o777); err != nil {
					return err
				}
				dest, err := os.Create(name)
				if err != nil {
					return err
				}

				n, err := io.Copy(dest, src)
				if err != nil {
					return err
				}
				if err := dest.Close(); err != nil {
					return err
				}

				atomic.AddInt64(&goFilesCount, 1)
				atomic.AddInt64(&goBytes, n)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		bar.Finish()
		log.Println(err)
	}
	bar.Finish()

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Unique modules:       % 7d -\n", len(latestVersions))
	fmt.Fprintf(os.Stderr, "Vendor paths:         % 7d -\n", vendor)
	fmt.Fprintf(os.Stderr, "Invalid names:        % 7d -\n", invalidName)
	fmt.Fprintf(os.Stderr, "Gone:                 % 7d -\n", gone)
	fmt.Fprintf(os.Stderr, "Invalid go.mod files: % 7d -\n", invalidGoMod)
	fmt.Fprintf(os.Stderr, "Mismatching go.mod:   % 7d -\n", mismatchedGoMod)
	fmt.Fprintf(os.Stderr, "GCS redirects:        % 7d -\n", gcsRedirect)
	fmt.Fprintf(os.Stderr, "No .go files:         % 7d -\n", noGoCode)
	fmt.Fprintf(os.Stderr, "No go.mod files:      % 7d =\n", noGoMod)
	fmt.Fprintf(os.Stderr, "                      -------\n")
	fmt.Fprintf(os.Stderr, "                      % 7d\n", good)
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "Downloaded %d bytes.\n", allBytes)
	fmt.Fprintf(os.Stderr, "Wrote %d Go files (%d bytes).\n", goFilesCount, goBytes)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}
