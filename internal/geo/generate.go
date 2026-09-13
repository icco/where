//go:build ignore

// Run with go generate ./internal/geo. Downloads public GeoNames data only.
package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func download(name string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", "https://download.geonames.org/export/dump/"+name, nil)
	if err != nil {
		panic(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		panic(resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s sha256 %x\n", name, sha256.Sum256(b))
	return b
}

func main() {
	regions := map[string]string{}
	for line := range strings.SplitSeq(string(download("admin1CodesASCII.txt")), "\n") {
		f := strings.Split(line, "\t")
		if len(f) >= 3 {
			regions[f[0]] = f[1]
		}
	}
	countries := map[string]string{}
	for line := range strings.SplitSeq(string(download("countryInfo.txt")), "\n") {
		f := strings.Split(line, "\t")
		if len(f) > 4 && !strings.HasPrefix(line, "#") {
			countries[f[0]] = f[4]
		}
	}
	data := download("cities15000.zip")
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic(err)
	}
	in, err := z.Open("cities15000.txt")
	if err != nil {
		panic(err)
	}
	defer in.Close()
	r := csv.NewReader(in)
	r.Comma = '\t'
	r.LazyQuotes = true
	out, err := os.Create("cities.csv.gz")
	if err != nil {
		panic(err)
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	w := csv.NewWriter(gz)
	count := 0
	for {
		f, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		if len(f) != 19 {
			panic("unexpected GeoNames schema")
		}
		if err := w.Write([]string{f[1], f[2], f[8], countries[f[8]], f[10], regions[f[8]+"."+f[10]], f[4], f[5], f[17], f[3]}); err != nil {
			panic(err)
		}
		count++
	}
	w.Flush()
	if err := w.Error(); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %d cities\n", count)
}
