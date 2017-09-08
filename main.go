package main

// type TMDb struct {
// 	apiKey string
// }
//
// func Init(apiKey string) *TMDb {
// 	return &TMDb{apiKey: apiKey}
// }
import (
	"github.com/Djoulzy/ImgStock/clog"
	"github.com/ryanbradynd05/go-tmdb"
)

var Conn = tmdb.Init("a0a1bc2a8a0f074c47fdae6efdeb5e04")

func main() {
	clog.LogLevel = 5
	clog.StartLogging = true

	conf, _ := Conn.GetConfiguration()
	clog.Trace("", "", "%s", conf)

	results, _ := Conn.SearchMovie("Fight Club", nil)
	clog.Trace("", "", "%v", results)

	fightClubInfo, _ := Conn.GetMovieImages(550, nil)
	for _, value := range fightClubInfo.Posters {
		clog.Trace("", "", "[%dx%d] - %s%s", value.Width, value.Height, conf.Images.BaseURL, value.FilePath)
	}
}
