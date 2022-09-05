package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	changeConLockCh = make(chan bool)
	getConLockCh    = make(chan bool)
)

func changeConLockState(state bool) {
	changeConLockCh <- state
}

func isConnLocked() bool {
	return <-getConLockCh
}

func connectionMonitior() {
	var conectionLocked = false

	for {
		select {
		case st := <-changeConLockCh:
			conectionLocked = st
			fmt.Println("Connectionn locked:", conectionLocked)
		case getConLockCh <- conectionLocked:
		}
	}
}

// ------------------------ mesagin between conection points ------------------
func listenPort(port int) error {
	l, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}

	con, err := l.Accept() // wait for connection
	if err != nil {
		return err
	}

	conProcess(&con)
	return nil
}

func connect(host string, port int) error {

	con, err := net.Dial("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}

	conProcess(&con)
	return nil
}

func conProcess(con *net.Conn) {
	var (
		getFromConnCh  = make(chan string)
		getFromStdInCh = make(chan string)
		signalCh       = make(chan struct{})
	)

	conRdWr := bufio.NewReadWriter(bufio.NewReader(*con), bufio.NewWriter(*con))
	stdInRd := bufio.NewReader(os.Stdin)

	//  ------------------- listen connection -----------------------
	go func(rdWr *bufio.ReadWriter) {

		for {
			str, err := rdWr.ReadString('\n')
			if err == io.EOF {
				signalCh <- struct{}{}
				close(getFromConnCh)
				return
			} else if err != nil {
				fmt.Println("Error reading message from connection:", err)
			} else {
				getFromConnCh <- str
			}
		}
	}(conRdWr)

	// ---------------- listen stdin ----------------
	go func(rd *bufio.Reader) {
		for {
			str, err := rd.ReadString('\n')
			if err != nil && err == io.EOF {
				fmt.Println("Stream closed - quiting")
				if len(str) > 0 {
					getFromStdInCh <- str
				}
				close(getFromStdInCh)
				signalCh <- struct{}{}
				return
			} else if err != nil {
				fmt.Println("Error reading message from stdin:", err)
			} else {
				getFromStdInCh <- str
			}
		}
	}(stdInRd)

	for {
		select {
		case str := <-getFromConnCh:
			fmt.Print(str)
		case str := <-getFromStdInCh:
			_, err := conRdWr.WriteString(str)
			if err != nil {
				fmt.Println("Error writing message message to connection:", err)
			}

			err = conRdWr.Flush()
			if err != nil {
				fmt.Println("Error flushing message to connection:", err)
			}
		case <-signalCh:
			// fmt.Println("Connection lost - quiting")
			return
		}
	}
}

// ------------------------ scan ports ------------------
func scanPorts(host string, portBegin int, portEnd int) {

	if portBegin > portEnd {
		portBegin, portEnd = portEnd, portBegin
	}

	for _, prot := range []string{"tcp"} {
		for i := portBegin; i <= portEnd; i++ {
			_, err := net.Dial(prot, fmt.Sprintf("%s:%d", host, i))
			if err == nil {
				fmt.Printf("Connection established %s %s:%d\n", prot, host, i)
			}
		}
	}
}

func main() {
	flagListen := flag.Bool("l", false, "Listen ports, usage: nc -l <port>")
	flagConnect := flag.Bool("c", false, "Connect ports, usage: nc -c <host> <port>")
	flagScan := flag.Bool("s", false, "Scan ports, usage: nc -s <host> <port begin>-<port end>")
	flag.Parse()

	if *flagListen {
		var (
			port int
		)

		if len(flag.Args()) != 1 {
			fmt.Println("Can not listen port, need number of port for listening")
			return
		}

		port, err := strconv.Atoi(flag.Args()[0])
		if err != nil {
			fmt.Println("Can not listen port? error reding port", err)
			return
		}

		err = listenPort(port)
		if err != nil {
			fmt.Println("Error listening port", port, "error", err)
		}
	} else if *flagConnect {
		var (
			host string
			port int
		)

		if len(flag.Args()) != 2 {
			fmt.Println("Can not connect, need host and port")
			return
		}
		host = flag.Args()[0]
		port, err := strconv.Atoi(flag.Args()[1])
		if err != nil {
			fmt.Println("Can not connect, error reding port", err)
			return
		}

		err = connect(host, port)
		if err != nil {
			fmt.Println("Error connecting", host, ":", port, "error", err)
		}

	} else if *flagScan {
		var (
			host      string
			portBegin int
			portEnd   int
		)

		if len(flag.Args()) != 2 {
			fmt.Println("Not enoough parameters. Usage: nc -s <host> <port begin>-<port end>")
			return
		}

		re := regexp.MustCompile("^\\d{1,4}-\\d{1,4}$")
		if !re.Match([]byte(flag.Args()[1])) {
			fmt.Println("Wrong param of ports for scaning. Usage: nc -s <host> <port begin>-<port end>.")
			fmt.Println("Got:", flag.Args()[1], "Need: <int>-<int>")
			return
		}

		host = flag.Args()[0]
		portBegin, _ = strconv.Atoi(strings.Split(flag.Args()[1], "-")[0])
		portEnd, _ = strconv.Atoi(strings.Split(flag.Args()[1], "-")[1])

		fmt.Printf("Scaning %s ports from %d to %d\n", host, portBegin, portEnd)
		scanPorts(host, portBegin, portEnd)
	}

}
