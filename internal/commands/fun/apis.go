package fun

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/salik/specter/internal/core"
	"github.com/salik/specter/internal/httpx"
)

func apiCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func handleAdvice(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := apiCtx()
	defer cancel()

	var resp struct {
		Slip struct {
			Advice string `json:"advice"`
		} `json:"slip"`
	}
	if err := httpx.GetJSON(ctx, "https://api.adviceslip.com/advice", &resp); err != nil || resp.Slip.Advice == "" {
		_ = c.Errorf("Could not fetch advice right now. Please try again later.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Advice").Description(resp.Slip.Advice).Build())
}

func handleFact(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := apiCtx()
	defer cancel()

	var resp struct {
		Text string `json:"text"`
	}
	if err := httpx.GetJSON(ctx, "https://uselessfacts.jsph.pl/api/v2/facts/random", &resp); err != nil || resp.Text == "" {
		_ = c.Errorf("Could not fetch a fact right now. Please try again later.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Random Fact").Description(resp.Text).Build())
}

func handleUrmom(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := apiCtx()
	defer cancel()

	var resp struct {
		Joke string `json:"joke"`
	}
	if err := httpx.GetJSON(ctx, "https://www.yomama-jokes.com/api/v1/jokes/random/", &resp); err != nil || resp.Joke == "" {
		_ = c.Errorf("Could not fetch a joke right now. Please try again later.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Yo Mama").Description(resp.Joke).Build())
}

func handleCat(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := apiCtx()
	defer cancel()

	var resp []struct {
		URL string `json:"url"`
	}
	if err := httpx.GetJSON(ctx, "https://api.thecatapi.com/v1/images/search", &resp); err != nil || len(resp) == 0 {
		_ = c.Errorf("Could not fetch a cat right now. Please try again later.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Cat").Image(resp[0].URL).Build())
}

func handleDog(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := apiCtx()
	defer cancel()

	var resp struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	}
	if err := httpx.GetJSON(ctx, "https://dog.ceo/api/breeds/image/random", &resp); err != nil || resp.Message == "" {
		_ = c.Errorf("Could not fetch a dog right now. Please try again later.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Dog").Image(resp.Message).Build())
}

func handleCapybara(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := apiCtx()
	defer cancel()

	var resp struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := httpx.GetJSON(ctx, "https://api.capy.lol/v1/capybara?json=true", &resp); err != nil || resp.Data.URL == "" {
		_ = c.Errorf("Could not fetch a capybara right now. Please try again later.", err)
		return
	}
	_ = c.Reply(c.Embed().Title("Capybara").Image(resp.Data.URL).Build())
}

func handleMeme(c *core.Context) {
	_ = c.Defer(false)
	ctx, cancel := apiCtx()
	defer cancel()

	// reddit.com/r/dankmemes/random.json returns a list with one listing.
	var listings []struct {
		Data struct {
			Children []struct {
				Data struct {
					Title string `json:"title"`
					URL   string `json:"url"`
					Over18 bool  `json:"over_18"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := httpx.GetJSON(ctx, "https://www.reddit.com/r/dankmemes/random.json", &listings); err != nil || len(listings) == 0 || len(listings[0].Data.Children) == 0 {
		_ = c.Errorf("Could not fetch a meme right now. Please try again later.", err)
		return
	}
	post := listings[0].Data.Children[0].Data
	if post.Over18 {
		_ = c.Errorf("The fetched meme was flagged NSFW; please try again.", nil)
		return
	}
	_ = c.Reply(c.Embed().Title(post.Title).Image(post.URL).Build())
}

func handleWiki(c *core.Context) {
	query := c.StringOpt("query", "")
	_ = c.Defer(false)
	ctx, cancel := apiCtx()
	defer cancel()

	endpoint := "https://en.wikipedia.org/api/rest_v1/page/summary/" + url.PathEscape(query)
	var resp struct {
		Title   string `json:"title"`
		Extract string `json:"extract"`
		ContentURLs struct {
			Desktop struct {
				Page string `json:"page"`
			} `json:"desktop"`
		} `json:"content_urls"`
		Thumbnail struct {
			Source string `json:"source"`
		} `json:"thumbnail"`
	}
	if err := httpx.GetJSON(ctx, endpoint, &resp); err != nil || resp.Extract == "" {
		_ = c.Errorf(fmt.Sprintf("No Wikipedia article found for %q.", query), err)
		return
	}
	b := c.Embed().Title(resp.Title).Description(resp.Extract).Thumbnail(resp.Thumbnail.Source)
	if resp.ContentURLs.Desktop.Page != "" {
		b.Footer(resp.ContentURLs.Desktop.Page)
	}
	_ = c.Reply(b.Build())
}
