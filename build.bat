set GO111MODULE=on
set CGO_ENABLED=0
set GOARCH=amd64
set GOPROXY=https://goproxy.cn,direct

go build -ldflags "-s -w -H windowsgui" -trimpath AutoStart.go
