package main

import (
	"fmt"
	"github.com/kardianos/osext"
)

func main() {
	filename, _ := osext.Executable()
	fmt.Println(filename)
}
