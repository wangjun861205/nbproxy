package nbproxy

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/wangjun861205/nbsoup"
)

var baseurl = "https://free-proxy-list.net/"

var pool = struct {
	m        map[string]bool
	l        sync.Mutex
	stopChan chan struct{}
	doneChan chan struct{}
	tickChan <-chan time.Time
}{
	make(map[string]bool),
	sync.Mutex{},
	make(chan struct{}),
	make(chan struct{}),
	time.Tick(30 * time.Minute),
}

type Client struct {
	proxyAddr string
	*http.Client
}

func init() {
	resp, err := http.Get(baseurl)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	root, err := nbsoup.Parse(content)
	if err != nil {
		log.Fatal(err)
	}
	trs, err := nbsoup.FindAll(root, `table[id="proxylisttable"].tbody.tr`)
	if err != nil {
		log.Fatal(err)
	}
	for _, tr := range trs {
		tds, err := nbsoup.FindAll(tr, `td`)
		if err != nil {
			log.Fatal(err)
		}
		addr := tds[0].Content
		port := tds[1].Content
		pool.m[addr+":"+port] = true
	}
	checkAll()
	validNum := getValidNum()
	if validNum == 0 {
		log.Fatal(ErrEmptyPool)
	}
	go start()
}

func start() {
	for {
		select {
		case <-pool.stopChan:
			close(pool.doneChan)
			return
		case <-pool.tickChan:
			pool.l.Lock()
			defer pool.l.Unlock()
			refreshPool()
			revive()
			checkAll()
			if getValidNum() == 0 {
				close(pool.stopChan)
				continue
			}
		}
	}
}

func ClosePool() {
	close(pool.stopChan)
}

func fetchAddr() ([]string, error) {
	resp, err := http.Get(baseurl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	root, err := nbsoup.Parse(content)
	if err != nil {
		return nil, err
	}
	trs, err := nbsoup.FindAll(root, `table[id="proxylisttable"].tbody.tr`)
	if err != nil {
		return nil, err
	}
	addrList := make([]string, len(trs))
	for i, tr := range trs {
		tds, err := nbsoup.FindAll(tr, `td`)
		if err != nil {
			return nil, err
		}
		addrList[i] = tds[0].Content + ":" + tds[1].Content
	}
	return addrList, nil
}

func checkAll() {
	var checkLock sync.Mutex
	invalidList := make([]string, 0, len(pool.m))
	var wg sync.WaitGroup
	for addr, status := range pool.m {
		if status {
			wg.Add(1)
			go func(a string) {
				fmt.Println(a)
				defer wg.Done()
				conn, err := net.DialTimeout("tcp", a, 10*time.Second)
				if err != nil {
					checkLock.Lock()
					defer checkLock.Unlock()
					invalidList = append(invalidList, a)
					return
				}
				conn.Close()
			}(addr)
		}
	}
	wg.Wait()
	for _, addr := range invalidList {
		pool.m[addr] = false
	}
}

func checkOne(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func getValidNum() int {
	var num int
	for _, status := range pool.m {
		if status {
			num += 1
		}
	}
	return num
}

func revive() {
	for addr, status := range pool.m {
		if !status {
			conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
			if err != nil {
				continue
			}
			defer conn.Close()
			pool.m[addr] = true
		}
	}
}

var ErrEmptyPool = errors.New("Empty pool")

func refreshPool() error {
	addrList, err := fetchAddr()
	if err != nil {
		return err
	}
	for _, addr := range addrList {
		if _, ok := pool.m[addr]; !ok {
			pool.m[addr] = true
		}
	}
	checkAll()
	validNum := getValidNum()
	if validNum == 0 {
		return ErrEmptyPool
	}
	return nil
}

var ErrPoolClosed = errors.New("proxy pool has closed")

func Pop() (*Client, error) {
	select {
	case <-pool.doneChan:
		return nil, ErrPoolClosed
	default:
		pool.l.Lock()
		defer pool.l.Unlock()
	ITER:
		for addr, status := range pool.m {
			if status && checkOne(addr) {
				url, err := url.Parse("http://" + addr)
				if err != nil {
					return nil, err
				}
				client := &http.Client{
					Timeout: 30 * time.Second,
					Transport: &http.Transport{
						Proxy: http.ProxyURL(url),
					},
				}
				delete(pool.m, addr)
				return &Client{addr, client}, nil
			} else {
				pool.m[addr] = false
				continue
			}
		}
		err := refreshPool()
		if err != nil {
			return nil, err
		}
		goto ITER
	}
}

func Put(c *Client) error {
	select {
	case <-pool.doneChan:
		return ErrPoolClosed
	default:
		if checkOne(c.proxyAddr) {
			pool.l.Lock()
			defer pool.l.Unlock()
			pool.m[c.proxyAddr] = true
		}
		return nil

	}
}
