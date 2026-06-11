module github.com/synacktiv/octoscan

go 1.25.0

// well I have a PR that is not merged: https://github.com/rhysd/actionlint/pull/332
// and I can"t use go install with replace directive: https://github.com/golang/go/issues/44840
// do you have any idea ?
replace github.com/rhysd/actionlint => github.com/AaronDewes/actionlint v0.0.0-20260611085532-a3b3456d02ae

require (
	github.com/docopt/docopt-go v0.0.0-20180111231733-ee0de3bc6815
	github.com/fatih/color v1.19.0
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/osv-scanner v1.9.2
	github.com/hashicorp/go-version v1.9.0
	github.com/rhysd/actionlint v1.7.12
	golang.org/x/oauth2 v0.36.0
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/mattn/go-shellwords v1.0.13 // indirect
	github.com/package-url/packageurl-go v0.1.6 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.5 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
)
