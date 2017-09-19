package MovieDB

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
	"os"
	"unicode/utf8"

	"github.com/Djoulzy/Tools/clog"
	curl "github.com/andelf/go-curl"
	"github.com/ryanbradynd05/go-tmdb"
)

type DataSource interface {
	GetTMDBKey() string
	GetCacheDir() string
}

type MDB struct {
	conn        *tmdb.TMDb
	config      *tmdb.Configuration
	baseURL     string
	posterSizes []string
	cacheDir    string
}

var conn = tmdb.Init("a0a1bc2a8a0f074c47fdae6efdeb5e04")
var conf *tmdb.Configuration

func (DB *MDB) memoryWriter(ptr []byte, userdata interface{}) bool {
	if ptr != nil {
		buffer := userdata.(*bytes.Buffer)
		buffer.Write(ptr)
	}
	return true
}

func (DB *MDB) cacheBuffer(buffer *bytes.Buffer, cacheID string, categorie string, fileType string) (string, error) {
	path := fmt.Sprintf("%s/%s", DB.cacheDir, categorie)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(path, os.ModePerm)
	}

	filename := fmt.Sprintf("%s/%s.%s", path, cacheID, fileType)
	file, _ := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0777)
	defer file.Close()

	if _, err := file.Write(buffer.Bytes()); err != nil {
		return "", errors.New("Can' write cache data")
	}
	finfo, _ := file.Stat()
	fsize := finfo.Size()
	clog.Info("MDB", "cacheBuffer", "Storing %s (%d)", filename, fsize)
	return filename, nil
}

func (DB *MDB) checkCache(cacheID string, categorie string, fileType string) (string, error) {
	if _, err := os.Stat(DB.cacheDir); os.IsNotExist(err) {
		os.MkdirAll(DB.cacheDir, os.ModePerm)
	}

	file := fmt.Sprintf("%s/%s/%s.%s", DB.cacheDir, categorie, cacheID, fileType)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return "", errors.New("No Data Found")
	}
	clog.Info("MDB", "checkCache", "Data found in cache: %s", file)
	return file, nil
}

func (DB *MDB) fetch(url string, buffer *bytes.Buffer) bool {
	easy := curl.EasyInit()
	defer easy.Cleanup()

	if easy != nil {
		easy.Setopt(curl.OPT_URL, url)
		easy.Setopt(curl.OPT_HEADER, 0)
		easy.Setopt(curl.OPT_WRITEFUNCTION, DB.memoryWriter)
		easy.Setopt(curl.OPT_WRITEDATA, buffer)

		clog.Info("MDB", "fetch", "Fetching %s", url)
		if err := easy.Perform(); err != nil {
			clog.Error("MDB", "fetch", "ERROR: %v\n", err)
			return false
		}

		code, _ := easy.Getinfo(curl.INFO_RESPONSE_CODE)
		if code == 200 {
			return true
		}
		clog.Error("MDB", "fetch", "ERROR CODE: %v", code)
		return false
	}
	clog.Error("MDB", "fetch", "cURL init problems ... Halting")
	log.Fatal()
	return false
}

func (DB *MDB) find(movieName string, size string, year string) (string, error) {
	if utf8.ValidString(movieName) {
		// rune, size := utf8.DecodeLastRuneInString(movieName)
		// clog.Trace("", "", "%s %d", rune, size)
	}
	var options = make(map[string]string)
	options["year"] = year
	options["include_adult"] = "true"
	results, err := DB.conn.SearchMovie(movieName, options)
	if err != nil {
		return "", err
	}
	if len(results.Results) == 0 {
		clog.Warn("MDB", "find", "Searching for '%s' year: %s, No Data Found", movieName, options["year"])
		return "", errors.New("No Data Found")
	}
	movie := results.Results[0]

	clog.Debug("MDB", "find", "Searching for '%s' year: %s, Found: '%v' [TmdbID:%d]", movieName, options["year"], movie.Title, movie.ID)
	filePath := fmt.Sprintf("%s%s%s", DB.baseURL, size, movie.PosterPath)

	return filePath, nil
}

func (DB *MDB) makeID(movieName string, year string) string {
	tmp := sha1.Sum([]byte(fmt.Sprintf("%s|%s", year, movieName)))
	return fmt.Sprintf("%x", tmp)
}

func (DB *MDB) GetSynopsys(movieName string, year string) (string, error) {
	var url string
	var err error
	var options = make(map[string]string)

	id := DB.makeID(string(movieName), year)
	url, err = DB.checkCache(id, "syn", "html")
	if err != nil {
		options["year"] = year
		options["include_adult"] = "true"
		results, _ := DB.conn.SearchMovie(movieName, options)
		movie := results.Results[0]

		options["language"] = "fr-FR"
		movieInfos, _ := DB.conn.GetMovieInfo(movie.ID, options)
		tmpBuff := bytes.NewBufferString(movieInfos.Overview)

		cachefile, _ := DB.cacheBuffer(tmpBuff, id, "syn", "html")
		return cachefile, nil
	} else {
		return url, nil
	}
}

func (DB *MDB) GetArtwork(movieName string, size string, year string) (string, error) {
	var buffer *bytes.Buffer
	var url string
	var err error

	id := DB.makeID(movieName, year)
	url, err = DB.checkCache(id, size, "jpg")
	if err != nil {
		url, err = DB.find(movieName, size, year)
		if err != nil {
			return "", errors.New("No Artwork Found")
		}
		buffer = new(bytes.Buffer)
		DB.fetch(url, buffer)
		cachefile, _ := DB.cacheBuffer(buffer, id, size, "jpg")
		return cachefile, nil
	} else {
		return url, nil
	}
}

func Init(appConf DataSource) *MDB {
	TMDB_Key := appConf.GetTMDBKey()
	DB := &MDB{
		conn:     tmdb.Init(TMDB_Key),
		cacheDir: appConf.GetCacheDir(),
	}

	conf, err := DB.conn.GetConfiguration()
	if err != nil {
		clog.Fatal("MDB", "Start", err)
	}
	DB.config = conf
	DB.baseURL = conf.Images.BaseURL
	DB.posterSizes = conf.Images.PosterSizes

	return DB
}
