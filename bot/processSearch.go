package bot

import (
	"github.com/XiaoMengXinX/Music163Api-Go/api"
)

func processSearch(keyword string, msg incomingMsg) (err error) {
	if keyword == "" {
		_, err = replyText(msg, inputKeyword)
		return err
	}
	searchResult, _ := api.SearchSong(data, api.SearchSongConfig{
		Keyword: keyword,
		Limit:   10,
	})
	if len(searchResult.Result.Songs) == 0 {
		_, err = replyText(msg, noResults)
		return err
	}
	var items []searchItem
	for i := 0; i < len(searchResult.Result.Songs) && i < 8; i++ {
		song := searchResult.Result.Songs[i]
		var songArtists string
		for j, artist := range song.Artists {
			if j == 0 {
				songArtists = artist.Name
			} else {
				songArtists = songArtists + "/" + artist.Name
			}
		}
		items = append(items, searchItem{
			MusicID: song.Id,
			Name:    song.Name,
			Artists: songArtists,
		})
	}
	_, err = replyCard(msg, searchCard(keyword, items))
	return err
}
