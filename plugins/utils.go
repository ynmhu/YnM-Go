package plugins

import (
	"regexp"
	"strings"

    "github.com/ynmhu/YnM-Go/irc"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// cleanJokeText megtisztítja a nyers HTML-ből kinyert szöveget
func cleanJokeText(text string) string {
	reStyle := regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)
	text = reStyle.ReplaceAllString(text, "")

	reScript := regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`)
	text = reScript.ReplaceAllString(text, "")

	reTags := regexp.MustCompile(`<[^>]*>`)
	text = reTags.ReplaceAllString(text, "")

	reCss := regexp.MustCompile(`[A-Za-z]\.vbx2(:link|:active|:visited|:hover)?`)
	text = reCss.ReplaceAllString(text, "")

	reCurly := regexp.MustCompile(`\{[^}]*\}`)
	text = reCurly.ReplaceAllString(text, "")

	t := transform.Chain(norm.NFC)
	result, _, err := transform.String(t, text)
	if err == nil {
		text = result
	}

	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return text
}

func JokeOnTickLogic(p *JokePlugin) []irc.Message {
    // itt van a közös logika
    return nil
}