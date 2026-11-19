package phonesync

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	psync "kando/packages/sync"
)

// soundSource is fakeSource's library with a film of sound alone besides, an episode's mp3.
type soundSource struct{ fakeSource }

func (f soundSource) Films(ctx context.Context, lib Library) ([]Film, error) {
	films, err := f.fakeSource.Films(ctx, lib)
	if err != nil {
		return nil, err
	}
	return append(films, Film{ID: "episode", Title: "Episode", Source: "web", Sound: true,
		Video: filepath.Join(f.dir, "episode.mp3"), Subtitles: filepath.Join(f.dir, "episode.bilingual.srt")}), nil
}

// A film of sound alone is listed to a phone that asks for such films, with its size, and its file is served as the sound it is, which the phone names it by; a phone from before them, which would keep it as an MP4, is not listed it.
func TestServesAFilmOfSoundToAPhoneThatAsks(t *testing.T) {
	lib := t.TempDir()
	for name, data := range map[string]string{"talk.mp4": "0123456789abcdefghij", "talk.bilingual.srt": "x", "episode.mp3": "ID3 sound", "episode.bilingual.srt": "x"} {
		if err := os.WriteFile(filepath.Join(lib, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srv, err := Start(Config{Dir: t.TempDir(), Name: "Test Mac", Source: soundSource{fakeSource{dir: lib}}, Addrs: loopback, Logf: testLogf(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	ctx := context.Background()
	c := client(t, srv, "")
	res, err := pair(t, c, srv.NewOffer().Code)
	if err != nil {
		t.Fatal(err)
	}
	films, err := c.Films(ctx, "lib1")
	if err != nil || len(films) != 2 {
		t.Fatalf("films %+v, %v", films, err)
	}
	if f := films[1]; f.ID != "episode" || !f.Sound || f.VideoSize != int64(len("ID3 sound")) {
		t.Fatalf("film %+v", f)
	}
	part, err := c.Video(ctx, "episode", 0, -1, "")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(part.Body)
	part.Body.Close()
	if string(body) != "ID3 sound" || part.ContentType != "audio/mpeg" || psync.Extension(part.ContentType, "") != ".mp3" {
		t.Fatalf("served %q as %q", body, part.ContentType)
	}

	// The films' path as a phone from before films of sound asks for it, with the films that keep no video.
	old := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	t.Cleanup(old.CloseIdleConnections)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+srv.Addrs()[0].String()+psync.FilmsPath("lib1")+"?"+psync.QueryWith+"="+psync.WithSubtitlesOnly, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+res.Token)
	resp, err := old.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var listed psync.Films
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil || resp.StatusCode != http.StatusOK || len(listed.Films) != 1 || listed.Films[0].ID != "talk" {
		t.Fatalf("an older phone's films %d %+v, %v", resp.StatusCode, listed, err)
	}
}
