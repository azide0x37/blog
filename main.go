package main

import (
	"flag"
	"fmt"
	"log"
	_ "net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/kjk/notionapi"
	"github.com/kjk/notionapi/caching_downloader"
)

var (
	analyticsCode = "UA-194516-1"

	flgRedownloadNotion bool
	flgRedownloadPage   string
	flgDeploy           bool
	flgPreview          bool
	flgPreviewOnDemand  bool
	flgVerbose          bool
	flgNoCache          bool
)

func parseCmdLineFlags() {
	flag.BoolVar(&flgVerbose, "verbose", false, "if true, verbose logging")
	flag.BoolVar(&flgNoCache, "no-cache", false, "if true, disables cache for downloading notion pages")
	flag.BoolVar(&flgDeploy, "deploy", false, "if true, build for deployment")
	flag.BoolVar(&flgPreview, "preview", false, "if true, runs caddy and opens a browser for preview")
	flag.BoolVar(&flgPreviewOnDemand, "preview-on-demand", false, "if true runs the browser for local preview")
	flag.BoolVar(&flgRedownloadNotion, "redownload-notion", false, "if true, re-downloads content from notion")
	flag.StringVar(&flgRedownloadPage, "redownload-page", "", "if given, redownloads content for one page")
	flag.Parse()
}

func rebuildAll(d *caching_downloader.Downloader) *Articles {
	regenMd()
	loadTemplates()
	articles := loadArticles(d)
	readRedirects(articles)
	netlifyBuild(articles)
	return articles
}

// caddy -log stdout
func runCaddy() {
	cmd := exec.Command("caddy", "-log", "stdout")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func stopCaddy(cmd *exec.Cmd) {
	cmd.Process.Kill()
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}

func preview() {
	go func() {
		time.Sleep(time.Second * 1)
		openBrowser("http://localhost:8080")
	}()
	runCaddy()
}

func eventObserver(ev interface{}) {
	switch v := ev.(type) {
	case caching_downloader.EventError:
		lg(v.Error)
	case caching_downloader.EventDidDownload:
		lg("'%s' : downloaded in %s\n", v.PageID, v.Duration)
	case caching_downloader.EventDidReadFromCache:
		// TODO: only verbose
		lg("'%s' : read from cache in %s\n", v.PageID, v.Duration)
	case caching_downloader.EventGotVersions:
		lg("downloaded info about %d versions in %s\n", v.Count, v.Duration)
	}
}

func main() {
	parseCmdLineFlags()
	os.MkdirAll("netlify_static", 0755)

	client := &notionapi.Client{}
	//client.Logger = os.Stdout
	cache, err := caching_downloader.NewDirectoryCache(cacheDir)
	must(err)
	d := caching_downloader.New(cache, client)
	d.EventObserver = eventObserver
	d.RedownloadNewerVersions = true
	d.NoReadCache = flgNoCache

	// make sure this happens first so that building for deployment is not
	// disrupted by the temporary testing code we might have below
	if flgDeploy {
		rebuildAll(d)
		return
	}

	if false {
		testNotionToHTMLOnePage(d, "dfbefe6906a943d8b554699341e997b0")
		os.Exit(0)
	}

	articles := rebuildAll(d)

	if flgPreview {
		preview()
		return
	}

	if true || flgPreviewOnDemand {
		startPreviewOnDemand(articles)
		return
	}
}
