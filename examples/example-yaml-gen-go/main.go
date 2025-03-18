package main

import (
	"context"
	_ "embed"

	"github.com/goaux/headline"
	"github.com/takumakei/yaml-gen-go/generator"
)

//go:embed main.tmpl
var template string

//go:embed usage.md
var usage string

func main() {
	generator.Main(context.Background(), generator.Config{
		Use:      "example-yaml-gen-go",
		Short:    headline.Get(usage),
		Long:     usage,
		Version:  "v0.0.0-alpha.1",
		Template: template,
	})
}
