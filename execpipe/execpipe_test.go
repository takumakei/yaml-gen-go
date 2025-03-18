package execpipe_test

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/goaux/results"
	"github.com/takumakei/yaml-gen-go/execpipe"
)

func Example() {
	results.Must(execpipe.CheckPath("goimports"))

	src := "package main\nimport \"fmt\"\nfunc main(){\nfmt.Println(`hello world`,)}"
	out := new(bytes.Buffer)
	results.Must(execpipe.Run(out, strings.NewReader(src), "goimports"))
	fmt.Print(out.String())
	// Output:
	// package main
	//
	// import "fmt"
	//
	// func main() {
	// 	fmt.Println(`hello world`)
	// }
}
