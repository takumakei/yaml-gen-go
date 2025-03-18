package generator

type Model struct {
	Gen    Gen
	Input  Input
	Output Output
}

type Gen struct {
	Name    string
	Version string
}

type Input struct {
	Path string
	Data any
}

type Output struct {
	Package string
}
