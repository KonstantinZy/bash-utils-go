# Some simple console utils written with GO

## ab
Simple analog of Apache HTTP server benchmarking tool
Start n goroutines and do m requests to typed host, as result print number of request statuses got by all goroutines
### Example
```
go run ab.go -n 10 -m 200 https://google.com
```
## nc
Simple analog of netcat util

### Usage
Listen ports: go run nc.go -l <port>
Connect ports, usage: nc -c <host> <port>
Scan ports, usage: nc -s <host> <port begin>-<port end>
### Example
For example you can try this

**Scan ports**

One terminal
```
go run nc.go -l 2055
```

Scan port from second terminal
```
go run nc.go -s 127.0.0.1 2000-3000
```
Result:
```
Scaning 127.0.0.1 ports from 2000 to 3000
Connection established tcp 127.0.0.1:2055
```
**Connect port**

One terminal
```
go run nc.go -l 2055
```

Connect from second terminal
```
go run nc.go -c 127.0.0.1 2055
```
And you can send messages from one terminal from another until one of them close connection

## multiwc
Analog bash util wc, but more for testing
This is just test several different approaches to the problem
### Example
Folder contains run.sh script which run all possible variants and show some measures like execution time, allocated memory etc.
You can run it and in file result.txt is gonna be result of work count of *.testdata files
