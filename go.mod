module github.com/user/slyds

go 1.25

require (
	github.com/panyam/templar v0.0.0
	github.com/spf13/cobra v1.10.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/panyam/goutils v0.1.10 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
)

replace github.com/panyam/templar => ./locallinks/newstack/templar/main
