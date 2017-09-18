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
	"crypto/sha1"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/Djoulzy/Tools/clog"
	"github.com/Djoulzy/Tools/config"
	curl "github.com/andelf/go-curl"
	"github.com/ryanbradynd05/go-tmdb"
	"github.com/valyala/fasthttp"
)

type Globals struct {
	LogLevel     int
	StartLogging bool
	HTTP_addr    string
	TMDB_Key     string
	CacheDir     string
}

type AppConfig struct {
	Globals
}

type MovieDB struct {
	conn        *tmdb.TMDb
	config      *tmdb.Configuration
	baseURL     string
	posterSizes []string
}

var conn = tmdb.Init("a0a1bc2a8a0f074c47fdae6efdeb5e04")
var conf *tmdb.Configuration
var cacheDir string

func (DB *MovieDB) handleError(ctx *fasthttp.RequestCtx, message string, status int) {
	ctx.SetStatusCode(status)
	fmt.Fprintf(ctx, "%s\n", message)
}

func (DB *MovieDB) sendBuffer(ctx *fasthttp.RequestCtx, buffer *bytes.Buffer) {
	ctx.Write(buffer.Bytes())
}

func (DB *MovieDB) sendBinary(ctx *fasthttp.RequestCtx, filepath string) {
	fasthttp.ServeFile(ctx, filepath)
}

func (DB *MovieDB) sendLogo(ctx *fasthttp.RequestCtx) {
	DB.sendBinary(ctx, "./tmdb.png")
}

func (DB *MovieDB) memoryWriter(ptr []byte, userdata interface{}) bool {
	if ptr != nil {
		buffer := userdata.(*bytes.Buffer)
		buffer.Write(ptr)
	}
	return true
}

func (DB *MovieDB) cacheBuffer(buffer *bytes.Buffer, cacheID string, categorie string, fileType string) bool {
	path := fmt.Sprintf("%s/%s", cacheDir, categorie)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(path, os.ModePerm)
	}

	filename := fmt.Sprintf("%s/%s.%s", path, cacheID, fileType)
	file, _ := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0777)
	defer file.Close()

	if _, err := file.Write(buffer.Bytes()); err != nil {
		return false
	}
	finfo, _ := file.Stat()
	fsize := finfo.Size()
	clog.Info("MovieDB", "cacheBuffer", "Storing %s (%d)", filename, fsize)
	return true
}

func (DB *MovieDB) checkCache(cacheID string, categorie string, fileType string) (string, error) {
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		os.MkdirAll(cacheDir, os.ModePerm)
	}

	file := fmt.Sprintf("%s/%s/%s.%s", cacheDir, categorie, cacheID, fileType)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return "", errors.New("No Data Found")
	}
	clog.Info("MovieDB", "checkCache", "Data found in cache: %s", file)
	return file, nil
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

func (DB *MovieDB) find(movieName string, size string, year string) (string, error) {
	if utf8.ValidString(movieName) {
		// rune, size := utf8.DecodeLastRuneInString(movieName)
		// clog.Trace("", "", "%s %d", rune, size)
	}
	var options = make(map[string]string)
	options["year"] = year
	results, err := DB.conn.SearchMovie(movieName, options)
	if err != nil {
		return "", err
	}
	if len(results.Results) == 0 {
		clog.Warn("MovieDB", "find", "Searching for '%s' year: %s, No Data Found", movieName, options["year"])
		return "", errors.New("No Data Found")
	}
	movie := results.Results[0]

	clog.Debug("MovieDB", "find", "Searching for '%s' year: %s, Found: '%v' [TmdbID:%d]", movieName, options["year"], movie.Title, movie.ID)
	filePath := fmt.Sprintf("%s%s%s", DB.baseURL, size, movie.PosterPath)

	return filePath, nil
}

func (DB *MovieDB) makeID(movieName string, year string) string {
	tmp := sha1.Sum([]byte(fmt.Sprintf("%s|%s", year, movieName)))
	return fmt.Sprintf("%x", tmp)
}

func (DB *MovieDB) getSynopsys(ctx *fasthttp.RequestCtx, query []string) {
	var url string
	var err error
	var options = make(map[string]string)

	movieName := query[1]

	var year string
	if len(query) > 2 {
		year = query[2]
	} else {
		year = ""
	}

	id := DB.makeID(string(movieName), year)
	url, err = DB.checkCache(id, "syn", "html")
	if err != nil {
		options["year"] = year
		results, _ := DB.conn.SearchMovie(movieName, options)
		movie := results.Results[0]

		options["language"] = "fr-FR"
		movieInfos, _ := DB.conn.GetMovieInfo(movie.ID, options)
		tmpBuff := bytes.NewBufferString(movieInfos.Overview)
		DB.sendBuffer(ctx, tmpBuff)
		ctx.SetContentType("text/html")
		DB.cacheBuffer(tmpBuff, id, "syn", "html")
	} else {
		ctx.SetContentType("text/html")
		DB.sendBinary(ctx, url)
	}
}

func (DB *MovieDB) getArtwork(ctx *fasthttp.RequestCtx, query []string) {
	var buffer *bytes.Buffer
	var url string
	var err error

	movieName := query[1]
	size := query[0]

	var year string
	if len(query) > 2 {
		year = query[2]
	} else {
		year = ""
	}

	id := DB.makeID(movieName, year)
	url, err = DB.checkCache(id, size, "jpg")
	if err != nil {
		url, err = DB.find(movieName, size, year)
		if err != nil {
			DB.sendLogo(ctx)
			return
		}
		buffer = new(bytes.Buffer)
		DB.fetch(url, buffer)
		ctx.SetContentType("image/jpeg")
		DB.sendBuffer(ctx, buffer)
		DB.cacheBuffer(buffer, id, size, "jpg")
	} else {
		ctx.SetContentType("image/jpeg")
		DB.sendBinary(ctx, url)
	}
}

func (DB *MovieDB) action(ctx *fasthttp.RequestCtx) {

	clog.Info("MovieDB", "action", "GET %s", ctx.Path())
	path := ctx.Path()
	query := strings.Split(string(path[1:]), "/")

	if len(query) < 2 {
		DB.handleError(ctx, "Bad Query", http.StatusNotFound)
		return
	}

	switch query[0] {
	case "favicon.ico":
		DB.handleError(ctx, "Not found", http.StatusNotFound)
		return
	case "syn":
		DB.getSynopsys(ctx, query)
	default:
		DB.getArtwork(ctx, query)
	}
}

func (DB *MovieDB) Start(appConf *AppConfig) {
	conf, err := DB.conn.GetConfiguration()
	if err != nil {
		clog.Fatal("MovieDB", "Start", err)
	}
	DB.config = conf
	DB.baseURL = conf.Images.BaseURL
	DB.posterSizes = conf.Images.PosterSizes

	clog.Info("MovieDB", "Start", "HTTP Listening on %s", appConf.HTTP_addr)
	err = fasthttp.ListenAndServe(appConf.HTTP_addr, DB.action)
	if err != nil {
		clog.Fatal("MovieDB", "Start", err)
	}
}

func main() {
	appConfig := &AppConfig{
		Globals{
			LogLevel:     5,
			StartLogging: true,
			HTTP_addr:    "localhost:9999",
		},
	}

	config.Load("MovieDB.ini", appConfig)
	clog.LogLevel = appConfig.LogLevel
	clog.StartLogging = appConfig.StartLogging
	cacheDir = appConfig.CacheDir

	DB := MovieDB{
		conn: tmdb.Init(appConfig.TMDB_Key),
	}

	DB.Start(appConfig)
}
