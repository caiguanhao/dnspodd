dnspodd
=======

Get DNSPOD domain info and save them to Gists.

```
# store configs and keys in environment variables
for v in PROXYURL GISTID; do printf "$v: " && read $v && export $v; done && \
  for v in DPEMAIL DPPASSWORD GHTOKEN; do printf "$v: " && read -s $v && echo && export $v; done

# build without asking
./build.sh

# clean
unset PROXYURL GISTID DPEMAIL DPPASSWORD GHTOKEN
```
