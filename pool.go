package nbproxy

import (
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/wangjun861205/nbsoup"
)

var baseurl = "https://free-proxy-list.net/"

type proxyPool struct {
	proxyAddrList []string
	locker        sync.Mutex
	tickChan      <-chan time.Time
	stopChan      chan struct{}
	doneChan      chan struct{}
}

func newProxyPool() (*proxyPool, error) {
	addrList, err := fetchAddr()
	if err != nil {
		return nil, err
	}
	return &proxyPool{addrList, sync.Mutex{}, time.Tick(10 * time.Minute), make(chan struct{}), make(chan struct{})}, nil
}

func (p *proxyPool) refresh() error {
	addrList, err := fetchAddr()
	if err != nil {
		return err
	}
	p.locker.Lock()
	defer p.locker.Unlock()
	tempMap := make(map[string]bool)
	for _, addr := range p.proxyAddrList {
		tempMap[addr] = true
	}
	for _, newAddr := range addrList {
		if !tempMap[newAddr] {
			p.proxyAddrList = append(p.proxyAddrList, newAddr)
		}
	}
	return nil
}

func (p *proxyPool) run() {
	for {
		select {
		case <-p.stopChan:
			close(p.doneChan)
			return
		case <-p.tickChan:
			p.refresh()
		}
	}
}

func (p *proxyPool) pop() (string, bool) {
	p.locker.Lock()
	defer p.locker.Unlock()
	if len(p.proxyAddrList) == 0 {
		return "", false
	}
	var addr string
	addr, p.proxyAddrList = p.proxyAddrList[0], p.proxyAddrList[1:]
	return addr, true
}

func (p *proxyPool) push(addr string) {
	p.locker.Lock()
	defer p.locker.Unlock()
	p.proxyAddrList = append(p.proxyAddrList, addr)
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
	trs, err := root.FindAll(`table[id="proxylisttable"].tbody.tr`)
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
