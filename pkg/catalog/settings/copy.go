package settings

import (
	"strings"
	"unicode/utf8"

	"github.com/pkg/errors"
)

// CopyKey is one piece of public site text that staff may override in
// storefront.copy. Only registered keys are accepted, so a misspelled key
// fails validation instead of silently changing nothing.
type CopyKey struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Help    string `json:"help,omitempty"`
	Kind    string `json:"kind"` // "text": one line; "markdown": rendered as Markdown
	Max     int    `json:"max"`  // characters
	Default string `json:"default"`
}

// CopyKeys lists the editable site text in the order the editor shows it.
var CopyKeys = []CopyKey{
	{Key: "header.statement", Label: "Header statement", Help: "Next to the wordmark on every page; also the landing page description for search engines.", Kind: "text", Max: 200,
		Default: "A lab for computation and intelligence as material and medium. Run by Manuel and Ramon."},
	{Key: "home.hero", Label: "Landing page headline", Help: "The large sentence above the list of experiments.", Kind: "text", Max: 240,
		Default: "These artifacts are the results of our research. Play with them, download the HTML, and drop them into your coding agent."},
	{Key: "home.search", Label: "Search placeholder", Kind: "text", Max: 80, Default: "Search experiments"},
	{Key: "callout.label", Label: "Support callout label", Kind: "text", Max: 60, Default: "Support the lab"},
	{Key: "callout.heading", Label: "Support callout heading", Kind: "text", Max: 120, Default: "Keep the lab running."},
	{Key: "callout.body", Label: "Support callout text", Kind: "markdown", Max: 2000,
		Default: "We prototype and explore, and share the work here. The lab is funded by people who want it to keep going. No ads, no clients, no investors."},
	{Key: "callout.button", Label: "Support callout button", Kind: "text", Max: 60, Default: "Support the lab →"},
	{Key: "runner.start", Label: "Start button", Help: "Starts an experiment for visitors who may run it.", Kind: "text", Max: 60, Default: "Start experiment →"},
	{Key: "runner.demo", Label: "Demo button", Help: "Starts a short demo for visitors without an account.", Kind: "text", Max: 60, Default: "Try it →"},
	{Key: "runner.signin", Label: "Sign-in button", Kind: "text", Max: 60, Default: "Sign in to run"},
	{Key: "earlyaccess.row", Label: "Early access, list", Help: "Shown in the list instead of Download; {date} is the end of early access.", Kind: "text", Max: 160,
		Default: "Early access for subscribers until {date}."},
	{Key: "earlyaccess.runner", Label: "Early access, app page", Help: "Shown next to the cover; {date} is the end of early access.", Kind: "text", Max: 200,
		Default: "In early access for subscribers until {date}. Subscribe to run it now."},
	{Key: "earlyaccess.button", Label: "Subscribe button", Kind: "text", Max: 60, Default: "Subscribe →"},
}

// CopyKeyByName returns the registered key, if any.
func CopyKeyByName(name string) (CopyKey, bool) {
	for _, k := range CopyKeys {
		if k.Key == name {
			return k, true
		}
	}
	return CopyKey{}, false
}

// CopyText returns the site text for key: the override in storefront.copy
// when it is not blank, else the registered default. Unknown keys return "".
func (d Document) CopyText(key string) string {
	if v := strings.TrimSpace(d.Storefront.Copy[key]); v != "" {
		return v
	}
	k, _ := CopyKeyByName(key)
	return k.Default
}

// SetCopy overrides one registered key; an empty value returns it to the
// default.
func (d *Document) SetCopy(key, value string) error {
	if _, ok := CopyKeyByName(key); !ok {
		return errors.Errorf("unknown copy key %q", key)
	}
	if d.Storefront.Copy == nil {
		d.Storefront.Copy = map[string]string{}
	}
	if strings.TrimSpace(value) == "" {
		delete(d.Storefront.Copy, key)
	} else {
		d.Storefront.Copy[key] = value
	}
	return validateCopy(d.Storefront.Copy)
}

func validateCopy(overrides map[string]string) error {
	for key, value := range overrides {
		k, ok := CopyKeyByName(key)
		if !ok {
			return errors.Errorf("storefront.copy: unknown key %q", key)
		}
		if !utf8.ValidString(value) || utf8.RuneCountInString(value) > k.Max {
			return errors.Errorf("storefront.copy.%s: at most %d characters", key, k.Max)
		}
		if k.Kind == "text" && strings.ContainsAny(value, "\r\n") {
			return errors.Errorf("storefront.copy.%s: one line only", key)
		}
	}
	return nil
}
