package nbproxy

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func TestPool(t *testing.T) {
	client, err := NewClient(10 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	content, err := client.Get("http://www.baidu.com")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))
}
