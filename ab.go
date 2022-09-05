package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type activeWorkers struct {
	RWM   *sync.RWMutex
	count int
}

func (aw *activeWorkers) add(cnt int) {
	aw.RWM.Lock()
	aw.count += cnt
	aw.RWM.Unlock()
}

func (aw *activeWorkers) reduce(cnt int) {
	aw.RWM.Lock()
	aw.count -= cnt
	if aw.count < 0 {
		aw.count = 0
	}
	aw.RWM.Unlock()
}

func (aw *activeWorkers) getCount() int {
	aw.RWM.Lock()
	cnt := aw.count
	aw.RWM.Unlock()
	return cnt
}

type requestData struct {
	statusCode int
	status     string
}

var (
	AW             = activeWorkers{count: 0, RWM: &sync.RWMutex{}}
	requestSending = true
	requestsDataCh = make(chan *requestData)
	url            = ""
)

// make testable functions
func makeRequest() {
	c, cancel := context.WithTimeout(context.Background(), time.Second*5) // timeout for request
	defer cancel()

	reqChan := make(chan *requestData)

	go func() {
		data, err := http.Get(url)
		if err == nil {
			reqD := &requestData{statusCode: data.StatusCode, status: data.Status}
			reqChan <- reqD
		} else {
			fmt.Println("Request error", err)
			cancel() // cancel context of main function
		}
	}()

	select {
	case <-c.Done():
		fmt.Println("Make request context error", c.Err())
		return
	case reqD := <-reqChan:
		requestsDataCh <- reqD
	}
}

func createWorkers(num int, requestMakeCh *chan bool, signalStop *chan bool, wg *sync.WaitGroup) error {

	for i := 1; i <= num; i++ {
		go func(requestMakeCh *chan bool, signalStop *chan bool, wg *sync.WaitGroup, wrkNum int) {
			defer wg.Done()

			fmt.Println("...Start worker", wrkNum)
			AW.add(1)
			time.Sleep(time.Millisecond * 200)

			for {
				select {
				case *requestMakeCh <- true:
					makeRequest()
				case <-*signalStop:
					fmt.Println("...Stop worker", wrkNum)
					AW.reduce(1)
					if AW.getCount() == 0 { // last worker close channel
						close(*requestMakeCh) // close it
						close(requestsDataCh)
					}
					return
				}
			}
		}(requestMakeCh, signalStop, wg, i)
	}

	return nil
}

func monitor(maxRequests int, signalStop *chan bool, requestMake *chan bool, wg *sync.WaitGroup) {
	defer wg.Done()

	var count = 0

	for <-*requestMake {
		count++
		if count >= maxRequests {
			workersCnt := AW.getCount()
			for i := 1; i <= workersCnt; i++ {
				*signalStop <- true
			}
			return
		}
	}
}

func resultCounter(wg *sync.WaitGroup) {
	defer wg.Done()

	cntStatuses := map[int]int{}

	for reqData := range requestsDataCh {
		_, ok := cntStatuses[reqData.statusCode]
		if ok {
			cntStatuses[reqData.statusCode] += 1
		} else {
			cntStatuses[reqData.statusCode] = 1
		}
	}

	fmt.Println("Result:")
	for status, cnt := range cntStatuses {
		fmt.Printf("Request with status %d - %d\n", status, cnt)
	}
}

func main() {

	var (
		signal        = make(chan bool)
		requestMakeCh = make(chan bool)
		wg            = &sync.WaitGroup{}
	)

	flgNumOfStreams := flag.Int("n", 3, "number of streams")
	flgAllReqCnt := flag.Int("m", 20, "number of all requests")
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Println("Give me domain name for testing")
		os.Exit(0)
	}

	url = flag.Args()[0]

	wg.Add(*flgNumOfStreams)
	createWorkers(*flgNumOfStreams, &requestMakeCh, &signal, wg)
	wg.Add(1)
	go monitor(*flgAllReqCnt, &signal, &requestMakeCh, wg)
	wg.Add(1)
	go resultCounter(wg)

	wg.Wait()

}
