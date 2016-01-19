#!/bin/bash

set -e

function str_to_array {
  eval "local input=\"\$$1\""
  input="$(echo "$input" | awk '
  {
    split($0, chars, "")
    for (i = 1; i <= length($0); i++) {
      if (i > 1) {
        printf(", ")
      }
      printf("\\\\\\\"%s\\\\\\\"", chars[i])
    }
  }
  ')"
  eval "$1=\"$input\""
}

function update_account {
  str_to_array DPEMAIL
  str_to_array DPPASSWORD
  str_to_array GHTOKEN
  str_to_array GISTID
  str_to_array PROXYURL
  awk "
  /DNSPOD_EMAIL/ {
    print \"var DNSPOD_EMAIL = strings.Join([]string{${DPEMAIL}}, \\\"\\\")\"
    next
  }
  /DNSPOD_PASSWORD/ {
    print \"var DNSPOD_PASSWORD = strings.Join([]string{${DPPASSWORD}}, \\\"\\\")\"
    next
  }
  /GITHUB_TOKEN/ {
    print \"var GITHUB_TOKEN = strings.Join([]string{${GHTOKEN}}, \\\"\\\")\"
    next
  }
  /GIST_ID/ {
    print \"var GIST_ID = strings.Join([]string{${GISTID}}, \\\"\\\")\"
    next
  }
  /PROXY_URL/ {
    print \"var PROXY_URL = strings.Join([]string{${PROXYURL}}, \\\"\\\")\"
    next
  }
  {
    print
  }
  " access.go > _access.go

  mv _access.go access.go
}

if test -z "$DPEMAIL"; then
  echo -n "Please type/paste your dnspod email: (will not be echoed) "
  read -s DPEMAIL
  echo
fi
if test -z "$DPPASSWORD"; then
  echo -n "Please type/paste your dnspod password: (will not be echoed) "
  read -s DPPASSWORD
  echo
fi
if test -z "$GHTOKEN"; then
  echo -n "Please type/paste your github token: (will not be echoed) "
  read -s GHTOKEN
  echo
fi
if test -z "$GISTID"; then
  echo -n "Please type/paste your gist ID: "
  read GISTID
fi
if ! env | grep -q PROXYURL; then
  echo -n "Please type/paste your proxy url: (optional, e.g. http://user:pass@host:12345) "
  read PROXYURL
fi
update_account

if test -n "$BUILD_DOCKER"; then
  docker-compose up
  docker-compose rm --force -v
else
  go build
fi

DPEMAIL="user@example.com"
DPPASSWORD="examplepassword"
GHTOKEN="0123456789abcde0123456789abcdeff01234567"
GISTID="00000000000000000000"
PROXYURL=""
update_account
