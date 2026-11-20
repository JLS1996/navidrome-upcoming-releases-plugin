package radar

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testArtwork() []byte {
	im := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			im.Set(x, y, color.RGBA{uint8(x * 25), uint8(y * 25), 80, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, im)
	return b.Bytes()
}
func TestArtworkRedirectDownloadCacheAndDisabledRender(t *testing.T) {
	s := published()
	c := cfg()
	c.CoverArt = true
	a := ArtworkState{}
	if !a.Reconcile(s, c, testNow) {
		t.Fatal("records not initialized")
	}
	calls := 0
	send := func(r HTTPRequest) (Response, error) {
		calls++
		if r.Headers["Authorization"] != "" {
			t.Fatal("artwork sent credentials")
		}
		if calls == 1 {
			return Response{Status: 307, Headers: map[string]string{"Location": "https://archive.org/download/fixture.png"}}, nil
		}
		return Response{Status: 200, Body: testArtwork()}, nil
	}
	if !a.Step(send, testNow) || calls != 1 {
		t.Fatal("redirect was not one callback")
	}
	if a.Step(send, testNow.Add(time.Second)) {
		t.Fatal("request pacing ignored")
	}
	a.Step(send, testNow.Add(2*time.Second))
	if calls != 2 || len(a.Images()) != 1 {
		t.Fatal("image not cached")
	}
	dir := t.TempDir()
	if err := SaveArtwork(dir, a); err != nil {
		t.Fatal(err)
	}
	restored, err := LoadArtwork(dir)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Step(send, testNow.Add(time.Hour)) {
		t.Fatal("cached image fetched again")
	}
	c.Covers = restored.Images()
	b, _ := Render(s, c, testNow)
	if !strings.Contains(string(b), `class="release has-cover has-artwork"`) || !strings.Contains(string(b), `class="release-cover"`) {
		t.Fatal("cached art absent from card")
	}
	c.CoverArt = false
	b, _ = Render(s, c, testNow)
	if strings.Contains(string(b), `class="release-cover"`) {
		t.Fatal("disabled art still embedded")
	}
}
func TestArtworkRejectsUnsafeRedirectsAndImages(t *testing.T) {
	for _, target := range []string{"http://archive.org/file.jpg", "https://archive.org.evil.example/file.jpg", "https://example.com/a", (&url.URL{Scheme: "https", Host: "archive.org", Path: "/a", User: url.UserPassword("fixture-user", "fixture-password")}).String(), "https://archive.org:444/a", "file:///etc/passwd"} {
		a := ArtworkState{}
		c := cfg()
		a.Reconcile(published(), c, testNow)
		a.Step(func(HTTPRequest) (Response, error) {
			return Response{Status: 307, Headers: map[string]string{"Location": target}}, nil
		}, testNow)
		if a.Problem == "" || a.Records[releaseID].URL != "" {
			t.Fatal("unsafe redirect accepted", target)
		}
	}
	for _, body := range [][]byte{[]byte(`<svg onload="alert(1)"></svg>`), bytes.Repeat([]byte("x"), maxArtworkBytes+1), {0xff, 0xd8, 0xff, 0}} {
		if _, err := artworkData(body); err == nil {
			t.Fatal("invalid image accepted")
		}
	}
	if safeArtworkData("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte("<svg/>"))) {
		t.Fatal("SVG accepted")
	}
	c := cfg()
	c.CoverArt = true
	c.Covers = map[string]string{releaseID: `data:image/png;base64,');alert(1)//`}
	b, _ := Render(published(), c, testNow)
	if strings.Contains(string(b), `class="release-cover"`) {
		t.Fatal("unsafe cached data reached HTML")
	}
}
func TestArtworkMissingAndFailureDoNotStarveOtherCovers(t *testing.T) {
	a := ArtworkState{}
	c := cfg()
	s := published()
	s.Entries[0].Releases = append(s.Entries[0].Releases, Release{ID: artistID, Type: "Album", Date: "2026-12-04"})
	a.Reconcile(s, c, testNow)
	first := ""
	a.Step(func(r HTTPRequest) (Response, error) {
		first = r.URL
		return Response{Status: 200, Body: []byte("not an image")}, nil
	}, testNow)
	second := ""
	a.Step(func(r HTTPRequest) (Response, error) { second = r.URL; return Response{Status: 404}, nil }, testNow.Add(2*time.Second))
	if first == second || second == "" {
		t.Fatal("one bad image blocked other covers")
	}
	if a.Step(func(HTTPRequest) (Response, error) {
		t.Fatal("missing cover re-fetched immediately")
		return Response{}, nil
	}, testNow.Add(time.Minute)) {
		t.Fatal("missing cover not cached")
	}
	a = ArtworkState{}
	a.Reconcile(published(), c, testNow)
	a.Step(func(HTTPRequest) (Response, error) {
		return Response{Status: 503, Headers: map[string]string{"Retry-After": "120"}}, nil
	}, testNow)
	if a.NextHTTP != testNow.Unix()+120 {
		t.Fatal("art service cooldown ignored")
	}
	a.Reconcile(State{Scope: "another-user"}, c, testNow)
	if len(a.Records) != 0 {
		t.Fatal("artwork retained across grant scopes")
	}
}
func TestArtworkCorruptCacheDoesNotSilentlyReset(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "covers.json"), []byte(`{"schema":1,"records":{"bad":null}}`), 0600)
	if _, err := LoadArtwork(dir); err == nil {
		t.Fatal("corrupt cache accepted")
	}
}
