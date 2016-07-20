package main

import (
	"io/ioutil"
	"os"
)

func main() {
	if bytes, err := ioutil.ReadFile("test.txt"); err == nil {
		ioutil.WriteFile("test.out.txt", bytes, os.ModePerm)
	}
}