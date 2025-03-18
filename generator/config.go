package generator

type Config struct {
	Use      string
	Short    string
	Long     string
	Version  string
	Template string

	DefaultInput   string
	DefaultOutput  string
	DefaultPackage string
	DefaultFormat  bool
}
