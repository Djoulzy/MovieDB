package main

// type TMDb struct {
// 	apiKey string
// }
//
// func Init(apiKey string) *TMDb {
// 	return &TMDb{apiKey: apiKey}
// }
import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"

	"github.com/Djoulzy/ImgStock/clog"
	curl "github.com/andelf/go-curl"
	"github.com/ryanbradynd05/go-tmdb"
	"github.com/valyala/fasthttp"
)

type MovieDB struct {
	conn        *tmdb.TMDb
	config      *tmdb.Configuration
	baseURL     string
	posterSizes []string
}

var conn = tmdb.Init("a0a1bc2a8a0f074c47fdae6efdeb5e04")
var conf *tmdb.Configuration

func (DB *MovieDB) sendBuffer(ctx *fasthttp.RequestCtx, buffer *bytes.Buffer) {
	ctx.SetContentType("image/jpeg")
	ctx.Write(buffer.Bytes())
}

func (DB *MovieDB) sendLogo(ctx *fasthttp.RequestCtx) {
	fasthttp.ServeFile(ctx, "./tmdb.png")
}

func (DB *MovieDB) memoryWriter(ptr []byte, userdata interface{}) bool {
	if ptr != nil {
		buffer := userdata.(*bytes.Buffer)
		buffer.Write(ptr)
	}
	return true
}

func (DB *MovieDB) fetch(url string, buffer *bytes.Buffer) bool {
	easy := curl.EasyInit()
	defer easy.Cleanup()

	if easy != nil {
		easy.Setopt(curl.OPT_URL, url)
		easy.Setopt(curl.OPT_HEADER, 0)
		easy.Setopt(curl.OPT_WRITEFUNCTION, DB.memoryWriter)
		easy.Setopt(curl.OPT_WRITEDATA, buffer)

		clog.Info("MovieDB", "fetch", "Fetching %s", url)
		if err := easy.Perform(); err != nil {
			clog.Error("MovieDB", "fetch", "ERROR: %v\n", err)
			return false
		}

		code, _ := easy.Getinfo(curl.INFO_RESPONSE_CODE)
		if code == 200 {
			return true
		}
		clog.Error("MovieDB", "fetch", "ERROR CODE: %v", code)
		return false
	}
	clog.Error("MovieDB", "fetch", "cURL init problems ... Halting")
	log.Fatal()
	return false
}

func (DB *MovieDB) find(movieName string) (string, error) {
	if utf8.ValidString(movieName) {
		rune, size := utf8.DecodeLastRuneInString(movieName)
		clog.Trace("", "", "%s %d", rune, size)
	}
	results, err := DB.conn.SearchMovie(movieName, nil)
	if err != nil {
		return "", err
	}
	if len(results.Results) == 0 {
		clog.Warn("MovieDB", "find", "Searching for '%s', No Data Found", movieName)
		return "", errors.New("No Data Found")
	}
	movieInfo := results.Results[0]
	clog.Debug("MovieDB", "find", "Searching for '%s', Found: '%v' [TmdbID:%d]", movieName, movieInfo.Title, movieInfo.ID)
	filePath := fmt.Sprintf("%sw185%s", DB.baseURL, movieInfo.PosterPath)

	return filePath, nil
}

func (DB *MovieDB) action(ctx *fasthttp.RequestCtx) {
	// var buffer *bytes.Buffer

	clog.Info("MovieDB", "action", "GET %s", ctx.Path())
	path := ctx.Path()
	query := strings.Split(string(path[1:]), "/")
	movieName := strings.Join(query, " ")

	url, err := DB.find(movieName)
	if err != nil {
		DB.sendLogo(ctx)
		return
	}
	clog.Trace("", "", "%s", url)
	// buffer = new(bytes.Buffer)
	// DB.fetch(url, buffer)
	// DB.sendBuffer(ctx, buffer)
}

func (DB *MovieDB) Start() {
	conf, err := DB.conn.GetConfiguration()
	if err != nil {
		clog.Fatal("MovieDB", "Start", err)
	}
	DB.config = conf
	DB.baseURL = conf.Images.BaseURL
	DB.posterSizes = conf.Images.PosterSizes

	err = fasthttp.ListenAndServe("localhost:9999", DB.action)
	if err != nil {
		clog.Fatal("MovieDB", "Start", err)
	}
}

func TEST() {
	d := encoding.Decoder{}
	s, _ := d.String("Ã©")
	clog.Trace("MovieDB", "Start", "%s", s)
}

func main() {
	clog.LogLevel = 5
	clog.StartLogging = true

	TEST()
	return
	DB := MovieDB{
		conn: tmdb.Init("a0a1bc2a8a0f074c47fdae6efdeb5e04"),
	}

	DB.Start()
}
