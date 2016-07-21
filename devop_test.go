package main_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestDevOP(t *testing.T) {
	os.Remove("./testData/testData")

	exec.Command("go", "build").Run()

	os.Chdir("./testData")

	DEVOP := exec.Command("../devop")

	DEVOP.Stderr = os.Stderr
	DEVOP.Stdout = os.Stdout

	DEVOP.Start()

	t_bytes := []byte("test ok")

	if err := ioutil.WriteFile("test.txt", t_bytes, os.ModePerm); err != nil {
		t.Error(err)
	}

	var found bool
	for i := 0; i < 500; i++ {
		time.Sleep(time.Millisecond)
		if n_bytes, err := ioutil.ReadFile("test.out.txt"); err == nil {
			found = true
			if !bytes.Equal(n_bytes, t_bytes) {
				t.Fail()
			}
			break
		}
	}

	if !found {
		t.Error("Can't read test.out.txt")
	}

	DEVOP.Process.Kill()
	DEVOP.Wait()

	//if _, err := os.Stat("test.out.txt"); err == nil {
	//	t.Error("test.out.txt file was not deleted")
	//}
	//
	//if _, err := os.Stat("test.txt"); err == nil {
	//	t.Error("test.txt file was not deleted")
	//}
	//
	//if _, err := os.Stat("testcmd"); err == nil {
	//	t.Error("testcmd file was not deleted")
	//}

	os.Remove("../devop")
	os.Remove("test.out.txt")
	os.Remove("test.txt")
	os.Remove("testcmd")
}

func TestName(t *testing.T) {

}
