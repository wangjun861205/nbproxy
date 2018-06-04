package nbproxy

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	pool   *proxyPool
	client *http.Client
}

func NewClient(timeout time.Duration) (*Client, error) {
	pool, err := newProxyPool()
	if err != nil {
		return nil, err
	}
	go pool.run()
	client := &http.Client{Timeout: timeout, Transport: http.DefaultTransport}
	return &Client{pool, client}, nil
}

func (c *Client) Get(urlStr string) ([]byte, error) {
	proxyAddr, ok := c.pool.pop()
	if !ok {
		return nil, errors.New("empty pool")
	}
	proxyURL, err := url.Parse("http://" + proxyAddr)
	if err != nil {
		return nil, err
	}
	c.client.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/67.0.3396.62 Safari/537.36")
	for i := 0; i < 3; i++ {
		resp, err := c.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		c.pool.push(proxyAddr)
		return content, nil
	}
	return nil, errors.New("http request failed")
}

func (c *Client) Close() {
	close(c.pool.stopChan)
	<-c.pool.doneChan
}
