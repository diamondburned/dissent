package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

const rev = "3912bae"
const prefix = "/xyz/diamondb/gtkcord4" // xyz.diamondb.gtkcord4

var (
	output = "."
	style  = "rounded" // outlined | rounded | sharp
	size   = 20
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <name>...\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}

	flag.StringVar(&output, "o", output, "output directory")
	flag.StringVar(&style, "s", style, "style of the icon (outlined, rounded, sharp)")
	flag.IntVar(&size, "px", size, "size of the icon")
	flag.Parse()

	icons := flag.Args()
	if len(icons) == 0 {
		flag.Usage()
		return
	}

	if err := os.MkdirAll(output, os.ModePerm); err != nil {
		log.Fatalln("Failed to create output directory:", err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	type downloadJob struct {
		url string
		dst string
	}

	jobs := make(chan downloadJob)
	var failed int32

	ctxJob, cancelJob := context.WithCancel(ctx)
	defer cancelJob()

	for i := 0; i < runtime.GOMAXPROCS(-1); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctxJob.Done():
					return
				case job := <-jobs:
					log.Println("Downloading", job.url, "to", job.dst)
					if err := download(ctx, job.url, job.dst); err != nil {
						atomic.AddInt32(&failed, 1)
						log.Println("Error:", err)
					}
				}
			}
		}()
	}

	tmpdir, err := os.MkdirTemp("", "gtkcord4-icons-*")
	if err != nil {
		log.Fatalln("failed to create tmpdir:", err)
	}
	defer os.RemoveAll(tmpdir)

	iconFiles := make([]IconFile, 0, len(icons))
	for _, icon := range icons {
		paths, _ := filepath.Glob(icon)
		if paths != nil {
			log.Println("Found multiple icon files", paths)
			for _, path := range paths {
				abs, _ := filepath.Abs(path)
				iconFiles = append(iconFiles, IconFile{
					Name: trimExt(filepath.Base(path)),
					Path: abs,
					Size: size,
				})
			}
			continue
		}

		filename := icon + ".svg"
		iconFiles = append(iconFiles, IconFile{
			Name: icon,
			Path: filename,
			Size: size,
		})

		url := svgURL(icon, style, size)
		dst := filepath.Join(tmpdir, filename)

		if _, err := os.Stat(dst); err == nil {
			log.Println("Skipping downloaded icon", icon)
		}

		select {
		case <-ctx.Done():
			return
		case jobs <- downloadJob{url, dst}:
			// ok
		}
	}

	cancelJob()
	wg.Wait()

	if failed > 0 {
		log.Fatalf("%d icons failed to download", failed)
	}

	gresource := generateGresourcesXML(iconFiles)
	log.Println("Compiling gresource...\n" + gresource)

	gresourceDst := filepath.Join(output, "icons.gresource")
	log.Println("Writing gresource to", gresourceDst)

	if err := generateGresources(ctx, tmpdir, gresource, gresourceDst); err != nil {
		log.Fatalln("failed to generate gresource:", err)
	}
}

func trimExt(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func download(ctx context.Context, url, dst string) error {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http: non-200 status: %s", resp.Status)
	}

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("os: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("io: %w", err)
	}

	return nil
}

func svgURL(icon, style string, size int) string {
	return fmt.Sprintf(
		"https://raw.githubusercontent.com/google/material-design-icons/%[1]s/symbols/web/%[2]s/materialsymbols%[3]s/%[2]s_%[4]dpx.svg",
		rev, icon, style, size,
	)
}

func generateGresources(ctx context.Context, baseDir, xml, output string) error {
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return fmt.Errorf("cannot get absolute path of output: os: %w", err)
	}

	xmlf, err := os.Create(filepath.Join(baseDir, "gresource.xml"))
	if err != nil {
		return fmt.Errorf("cannot make tmp gresource.xml: os: %w", err)
	}
	defer xmlf.Close()

	if _, err := xmlf.WriteString(xml); err != nil {
		return fmt.Errorf("cannot write to tmp gresource.xml: os: %w", err)
	}

	if err := xmlf.Close(); err != nil {
		return fmt.Errorf("cannot finalize tmp gresource.xml: os: %w", err)
	}

	cmd := exec.CommandContext(ctx, "glib-compile-resources", "--target", absOutput, xmlf.Name())
	cmd.Dir = baseDir

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			log.Println("glib-compile-resources stderr:", string(exitErr.Stderr))
		}
		return fmt.Errorf("glib-compile-resources: %w", err)
	}

	return nil
}

type IconFile struct {
	Name string
	Path string
	Size int
}

func generateGresourcesXML(icons []IconFile) string {
	var b strings.Builder
	fmt.Fprint(&b, `<?xml version="1.0" encoding="UTF-8"?>`+"\n")
	fmt.Fprint(&b, `<gresources>`+"\n")
	fmt.Fprintf(&b, `  <gresource prefix="%s">`+"\n", prefix+"/icons")
	for _, icon := range icons {
		alias := fmt.Sprintf("scalable/actions/%s-symbolic.svg", icon.Name)
		fmt.Fprintf(&b, `    <file alias=%q>%s</file>`+"\n", alias, icon.Path)
	}
	fmt.Fprint(&b, `  </gresource>`+"\n")
	fmt.Fprint(&b, `</gresources>`+"\n")
	return b.String()
}
