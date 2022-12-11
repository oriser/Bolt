package wolt

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/prometheus/common/log"
	"golang.org/x/net/html"
)

type WoltAddr struct {
	BaseAddr    string
	APIBaseAddr string

	baseAddrParsed *url.URL
	apiAddrParsed  *url.URL
}

type RetryConfig struct {
	HTTPMaxRetries       int
	HTTPMinRetryDuration time.Duration
	HTTPMaxRetryDuration time.Duration
}

func (w *WoltAddr) parse() error {
	u, err := url.Parse(w.BaseAddr)
	if err != nil {
		return fmt.Errorf("parse base addr: %w", err)
	}
	w.baseAddrParsed = u

	u, err = url.Parse(w.APIBaseAddr)
	if err != nil {
		return fmt.Errorf("parse api addr: %w", err)
	}
	w.apiAddrParsed = u
	return nil
}

type Group struct {
	woltAddrs WoltAddr
	prettyID  string
	id        string
	auth      string
	client    *http.Client
	headers   map[string]string
}

func newGroup(woltAddrs WoltAddr, retryConfig RetryConfig, id string) (*Group, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookiejar: %w", err)
	}

	client := retryablehttp.NewClient()
	client.HTTPClient.Jar = jar
	client.RetryWaitMax = retryConfig.HTTPMaxRetryDuration
	client.RetryWaitMin = retryConfig.HTTPMinRetryDuration
	client.RetryMax = retryConfig.HTTPMaxRetries
	client.Logger = nil
	client.RequestLogHook = func(logger retryablehttp.Logger, request *http.Request, i int) {
		if i != 0 {
			log.Errorf("Retrying request for %s (attempt %d)", request.URL.String(), i)
		}
	}

	if err = woltAddrs.parse(); err != nil {
		return nil, fmt.Errorf("parse wolt addrs: %w", err)
	}

	return &Group{
		woltAddrs: woltAddrs,
		prettyID:  id,
		client:    client.StandardClient(),
		headers: map[string]string{
			"User-Agent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.16; rv:84.0) Gecko/20100101 Firefox/84.0",
			"Origin":       woltAddrs.BaseAddr,
			"Content-Type": "application/json;charset=utf-8",
		}}, nil
}

func NewGroupWithExistingID(woltAddrs WoltAddr, retryConfig RetryConfig, id string) (*Group, error) {
	return newGroup(woltAddrs, retryConfig, id)
}

func isIDMatch(n *html.Node, id string) bool {
	if n.Type != html.ElementNode {
		return false
	}

	for _, attr := range n.Attr {
		if attr.Key == "id" && id == attr.Val {
			return true
		}
	}
	return false
}

func findByID(n *html.Node, id string) *html.Node {
	if isIDMatch(n, id) {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		res := findByID(c, id)
		if res != nil {
			return res
		}
	}

	return nil
}

func (g *Group) joinBaseAddr(p string) string {
	u := *g.woltAddrs.baseAddrParsed
	u.Path = path.Join(u.Path, p)
	return u.String()
}

func (g *Group) joinApiAddr(p string) string {
	u := *g.woltAddrs.apiAddrParsed
	u.Path = path.Join(u.Path, p)
	return u.String()
}

func (g *Group) assignIDFromHTML(data io.Reader) error {
	doc, err := html.Parse(data)
	if err != nil {
		return fmt.Errorf("html parse: %w", err)
	}

	scriptElement := findByID(doc, "bootstrap")
	if scriptElement == nil {
		return fmt.Errorf("find bootstrap script")
	}

	decodedScript, err := url.QueryUnescape(strings.TrimSpace(scriptElement.FirstChild.Data))
	if err != nil {
		return fmt.Errorf("decode script: %w", err)
	}

	gc, err := gabs.ParseJSON([]byte(decodedScript))
	if err != nil {
		return fmt.Errorf("parse bottstrap JSON: %w", err)
	}

	gc = gc.S("groupOrder", "order", "confirmedState", "id")
	if gc == nil {
		return fmt.Errorf("find group id from bootstrap JSON")
	}

	g.id = gc.Data().(string)
	return nil
}

func (g *Group) prepareReq(method, url string, body io.Reader, extraHeaders map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	for key, val := range g.headers {
		req.Header.Set(key, val)
	}

	for key, val := range extraHeaders {
		req.Header.Set(key, val)
	}

	return req, nil
}

func (g *Group) sendReq(req *http.Request) (*http.Response, error) {
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending https req: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got non 200 response: %d", resp.StatusCode)
	}

	return resp, nil
}

func (g *Group) joinByRealID() error {
	body := bytes.NewBuffer([]byte(`{"first_name":"Wolt Bot"}`))

	reqURL := g.joinApiAddr(fmt.Sprintf("/v1/group_order/guest/join/%s", g.id))
	if g.auth != "" {
		reqURL = g.joinApiAddr(fmt.Sprintf("/v1/group_order/join/%s", g.id))
	}

	req, err := g.prepareReq(
		"POST",
		reqURL,
		body,
		map[string]string{"Referer": g.woltAddrs.BaseAddr})
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	_, err = g.sendReq(req)
	if err != nil {
		return fmt.Errorf("join request http res: %w", err)
	}

	return nil
}

func (g *Group) Join() error {
	req, err := g.prepareReq("GET", g.joinBaseAddr(fmt.Sprintf("/en/group-order/%s/join", g.prettyID)), nil, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	resp, err := g.sendReq(req)
	if err != nil {
		return fmt.Errorf("getting http response: %w", err)
	}

	if err := g.assignIDFromHTML(resp.Body); err != nil {
		return fmt.Errorf("getting real group ID: %w", err)
	}

	return g.joinByRealID()
}

func (g *Group) Details() (*OrderDetails, error) {
	reqURL := g.joinApiAddr(fmt.Sprintf("/v1/group_order/guest/%s/participants/me", g.id))
	if g.auth != "" {
		reqURL = g.joinApiAddr(fmt.Sprintf("/v1/group_order/%s/participants/me", g.id))
	}

	b := bytes.NewBuffer([]byte(`{"subscribed":false}`))
	req, err := g.prepareReq("PATCH", reqURL, b, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	resp, err := g.sendReq(req)
	if err != nil {
		return nil, fmt.Errorf("details http res: %w", err)
	}

	output, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading output: %w", err)
	}

	gc, err := gabs.ParseJSON(output)
	if err != nil {
		return nil, fmt.Errorf("parse is active JSON (%s): %w", string(output), err)
	}

	return &OrderDetails{ParsedOutput: gc}, nil
}

func (g *Group) VenueDetails() (*VenueDetails, error) {
	details, err := g.Details()
	if err != nil {
		return nil, fmt.Errorf("get group details: %w", err)
	}

	venueID, err := details.VenueID()
	if err != nil {
		return nil, fmt.Errorf("get venue ID from details: %w", err)
	}

	req, err := g.prepareReq("GET", g.joinApiAddr(fmt.Sprintf("/v3/venues/%s", venueID)), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("prepare venue request: %w", err)
	}

	resp, err := g.sendReq(req)
	if err != nil {
		return nil, fmt.Errorf("send venue details request: %w", err)
	}

	output, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading output: %w", err)
	}

	gc, err := gabs.ParseJSON(output)
	if err != nil {
		return nil, fmt.Errorf("parse venue details JSON: %w", err)
	}

	return &VenueDetails{ParsedOutput: gc}, nil
}

func (g *Group) MarkAsReady() error {
	reqURL := g.joinApiAddr(fmt.Sprintf("/v1/group_order/guest/%s/participants/me", g.id))
	if g.auth != "" {
		reqURL = g.joinApiAddr(fmt.Sprintf("/v1/group_order/%s/participants/me", g.id))
	}

	b := bytes.NewBuffer([]byte(`{"status":"ready"}`))
	req, err := g.prepareReq("PATCH", reqURL, b, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	_, err = g.sendReq(req)
	if err != nil {
		return fmt.Errorf("mark as ready http res: %w", err)
	}

	return nil
}
