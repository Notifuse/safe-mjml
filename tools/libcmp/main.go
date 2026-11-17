// Command libcmp renders a list of MJML documents with one Go MJML library
// and writes each output, for a comparison with MJML's references.
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	boostport "github.com/Boostport/mjml-go"
	notifuse "github.com/Notifuse/safe-mjml/mjml"
	mjml2html "github.com/TcMits/mjml2html"
	mrmlgo "github.com/codedninja/mrmlgo"
	handsomefox "github.com/handsomefox/mjml-go"
	julez "github.com/julez-dev/mjmlgo/v2"
	mgmlcore "github.com/krozhkov/mgml/core"
	mgml "github.com/krozhkov/mgml/mjml"
	preslav "github.com/preslavrachev/gomjml/mjml"
)

var renderers = map[string]func(string) (string, error){
	"notifuse": func(s string) (string, error) { return notifuse.Render(s) },
	"preslav":  func(s string) (string, error) { return preslav.Render(s) },
	"mgml": func(s string) (string, error) {
		return mgml.MJML2Html(s, &mgmlcore.MJMLOptions{ValidationLevel: mgmlcore.ValidationLevelSoft, KeepComments: true})
	},
	"boostport": func(s string) (string, error) {
		return boostport.ToHTML(context.Background(), s, boostport.WithMinify(true), boostport.WithValidationLevel(boostport.Soft))
	},
	"handsomefox": func(s string) (string, error) {
		return handsomefox.ToHTML(context.Background(), s, handsomefox.WithMinify(true), handsomefox.WithValidationLevel(handsomefox.Soft))
	},
	"julez": func(s string) (string, error) {
		var b bytes.Buffer
		err := julez.NewRenderer().Render(context.Background(), &b, []byte(s))
		return b.String(), err
	},
	"tcmits": func(s string) (string, error) {
		return mjml2html.ToHTML(s, mjml2html.ValidationLevel("soft"))
	},
	"mrml": func(s string) (string, error) {
		opts, err := mrmlgo.NewParseOptions(nil)
		if err != nil {
			return "", err
		}
		defer opts.Destory()
		out, err := opts.ParseMJML(s)
		if err != nil {
			return "", err
		}
		defer out.Destory()
		return out.ToHTML()
	},
}

func main() {
	if os.Args[1] == "hostile" {
		hostile(os.Args[2], os.Args[3])
		return
	}
	if os.Args[1] == "bench" {
		bench(os.Args[2], os.Args[3:])
		return
	}
	lib, list, outDir := os.Args[1], os.Args[2], os.Args[3]
	render := renderers[lib]
	if render == nil {
		fmt.Fprintln(os.Stderr, "unknown library", lib)
		os.Exit(2)
	}
	f, err := os.Open(list)
	if err != nil {
		panic(err)
	}
	var docs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		docs = append(docs, sc.Text())
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}
	workers := runtime.NumCPU()
	if w := os.Getenv("WORKERS"); w != "" {
		fmt.Sscan(w, &workers)
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed, timedOut := 0, 0
	start := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				src, err := os.ReadFile(docs[i])
				if err != nil {
					panic(err)
				}
				type result struct {
					html string
					err  error
				}
				done := make(chan result, 1)
				go func() {
					defer func() {
						if r := recover(); r != nil {
							done <- result{err: fmt.Errorf("panic: %v", r)}
						}
					}()
					h, e := render(string(src))
					done <- result{h, e}
				}()
				var r result
				select {
				case r = <-done:
				case <-time.After(30 * time.Second):
					r = result{err: fmt.Errorf("timeout")}
					mu.Lock()
					timedOut++
					mu.Unlock()
				}
				name := filepath.Join(outDir, fmt.Sprintf("%05d", i))
				if r.err != nil {
					mu.Lock()
					failed++
					mu.Unlock()
					_ = os.WriteFile(name+".err", []byte(r.err.Error()), 0o644)
					continue
				}
				_ = os.WriteFile(name+".html", []byte(r.html), 0o644)
			}
		}()
	}
	for i := range docs {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	fmt.Printf("%s: %d documents, %d errors (%d timeouts), %s\n", lib, len(docs), failed, timedOut, time.Since(start).Round(time.Second))
}

// bench prints the median time of one render of each file, after a warm-up.
func bench(lib string, files []string) {
	render := renderers[lib]
	for _, file := range files {
		src, _ := os.ReadFile(file)
		for i := 0; i < 3; i++ {
			_, _ = render(string(src))
		}
		var times []time.Duration
		for i := 0; i < 15; i++ {
			start := time.Now()
			_, err := render(string(src))
			times = append(times, time.Since(start))
			if err != nil {
				fmt.Printf("%s %s error: %.80s\n", lib, filepath.Base(file), err)
				break
			}
		}
		sort.Slice(times, func(a, b int) bool { return times[a] < times[b] })
		fmt.Printf("%-12s %-30s %10.2f ms\n", lib, filepath.Base(file), float64(times[len(times)/2].Microseconds())/1000)
	}
}
