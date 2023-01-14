# Sheets-Test
## About
Sheets-test is my attempt to integrate with google sheets by reading from and making changes to sheets. \
The project is based on https://developers.google.com/sheets/api/quickstart/go

## Expectations
* burndown/burn up charts
* track debt payoff
* track savings for vacation/travel/house
* track plan for retirement

## commands to build starter project:
* mkdir sheets-test && cd sheets-test
* go mod init github.com/yourname/sheets-test # if you messed this up, run $`rm go.mod && rm go.sum`
* export GO111MODULE=on # to fix "io/fs: package io/fs is not in GOROOT (/usr/local/opt/go/libexec/src/io/fs)"
* go get google.golang.org/api/sheets/v4
* go get golang.org/x/oauth2/google
* (paste in the starter code)
* go mod tidy
* go run .