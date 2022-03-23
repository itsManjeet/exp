module golang.org/x/exp/cmd/govulncheck

go 1.17

require (
	github.com/client9/misspell v0.3.4
	golang.org/x/exp/vulncheck v0.0.0-20220307200941-a1099baf94bf
	golang.org/x/mod v0.6.0-dev.0.20211013180041-c96bc1413d57
	golang.org/x/tools v0.1.8
	golang.org/x/vuln v0.0.0-20220323192046-4fe4bee419ef
	honnef.co/go/tools v0.3.0-0.dev.0.20220306074811-23e1086441d2
	mvdan.cc/unparam v0.0.0-20211214103731-d0ef000c54e5
)

require (
	github.com/BurntSushi/toml v0.4.1 // indirect
	golang.org/x/exp/typeparams v0.0.0-20220218215828-6cf2b201936e // indirect
	golang.org/x/sys v0.0.0-20211213223007-03aa0b5f6827 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)

replace golang.org/x/exp/vulncheck => ../../vulncheck
