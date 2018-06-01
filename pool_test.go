package nbproxy

import (
	"fmt"
	"io/ioutil"
	"log"
	"testing"
)

func TestPool(t *testing.T) {
	defer ClosePool()
	client, err := Pop()
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Get("http://www.baidu.com")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))

}
