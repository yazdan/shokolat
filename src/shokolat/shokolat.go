package main

import (
	"flag"
	"github.com/yazdan/goproxy"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"fmt"
	"regexp"
	"bufio"
	"strings"
	"io"
)



type readCloser struct {
    io.Reader
    io.Closer

}


var (
	StartBodyTagMatcher = regexp.MustCompile(`(?i:<body.*>)`)
	ProxyControlPort    = "8080"
	// TODO: make this UUID generated on startup, accessed via singleton?
	ProxyExceptionString = "LOL-WHUT-JUST-DOIT-DOOD"
)

// Parse a file of regular expressions, ignoring comments/whitespace
func GetRegexlist(filename string) ([]*regexp.Regexp, error) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening %s: %q", filename, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var list []*regexp.Regexp = make([]*regexp.Regexp, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// ignore blank/whitespace lines and comments
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			// add ignore case option to regex and compile it
			if r, err := regexp.Compile("(?i)" + line); err == nil {
				list = append(list, r)
			} else {
				log.Fatalf("Invalid pattern: %q", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading %s: %q", filename, err)
	}
	return list, nil
}

func main() {
	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	noAria := flag.Bool("noaria2", false, "should every proxy request be logged to stdout")
	docRoot := flag.String("root", "./cache", "document root directory")
	address := flag.String("http", ":8080", `HTTP service address (e.g., ":8080")`)
	cacheListFilename := flag.String("cl", "cachelist.txt", "file of regexes to cache request urls")
	
	flag.Parse()

	cacheList, clErr := GetRegexlist(*cacheListFilename)
	if clErr != nil {
		log.Fatalf("Could not load chache list. Error: %s", clErr)
	}

	err := os.MkdirAll(*docRoot, 0755)
	if err != nil {
		log.Fatalf("Could not make cache dir. Error: %s", err)
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = *verbose

	proxy.OnRequest(reqMethodIs("GET", "HEAD")).DoFunc(
		func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			ctx.Logf("Url: %s", ctx.Req.URL)
			ctx.Logf("Range : %s",ctx.Req.Header.Get("Range"))
			
			ctx.Logf("%s", cacheList)

			for _, cRegex := range cacheList {
				if cRegex.MatchString(req.URL.String()) {
					_, urlName := path.Split(ctx.Req.URL.Path)
					filename := path.Join(*docRoot, urlName)
					if !exists(filename) {
			
						return req, goproxy.NewResponse(req,
							goproxy.ContentTypeHtml, http.StatusForbidden,
							fmt.Sprintf(`<html>
					    <head><title>BLOCKED</title></head>
					    <body>
					        <h1>File is not cached!</h1>
					        <hr />
					    </body>
					</html>`))
					}

					//bytes, err := ioutil.ReadFile(filename)
					file, err := os.Open( filename ) 

					if err != nil {
						ctx.Warnf("%s", err)
						return req, nil
					}
					stat,_ := file.Stat()
					resp := goproxy.NewResponseReader(req, "application/octet-stream",
						http.StatusOK, readCloser{bufio.NewReader(file), file}, stat.Size())
					ctx.Logf("return response from local %s", filename)
					return req, resp
				}
			}
			ctx.Logf("Not Matched!")
			return req, nil
		})

	proxy.OnResponse(respReqMethodIs("GET", "HEAD")).Do(
		goproxy.HandleBytes(
			func(b []byte, ctx *goproxy.ProxyCtx) []byte {
				if ctx.Req.Method != "GET" || hasRespHeader(ctx.Resp, "Location") {
					return b
				}
				if *noAria {
					filename := path.Join(*docRoot, ctx.Req.URL.Path)
					for _, cRegex := range cacheList {
						if cRegex.MatchString(ctx.Req.URL.String()) {
							if exists(filename) {
								return b
							}

							dir := path.Dir(filename)
							err := os.MkdirAll(dir, 0755)
							if err != nil {
								ctx.Warnf("cannot create directory: %s", dir)
							}

							err = ioutil.WriteFile(filename, b, 0644)
							if err != nil {
								ctx.Warnf("cannot write file: %s", filename)
							}

							ctx.Logf("save cache to %s", filename)
						}
					}
				}

				return b
			}))
	log.Fatal(http.ListenAndServe(*address, proxy))
}

func reqMethodIs(methods ...string) goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		for _, method := range methods {
			if req.Method == method {
				return true
			}
		}
		return false
	}
}

func respReqMethodIs(methods ...string) goproxy.RespConditionFunc {
	return func(resp *http.Response, ctx *goproxy.ProxyCtx) bool {
		for _, method := range methods {
			if resp.Request.Method == method {
				return true
			}
		}
		return false
	}
}

func hasRespHeader(resp *http.Response, header string) bool {
	_, ok := resp.Header[header]
	return ok
}

func exists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

