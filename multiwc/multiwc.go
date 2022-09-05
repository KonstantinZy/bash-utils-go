package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

/* --------------------------- for Mutex logic ---------------------------- */
type MainWordsCount struct {
	count int
	*sync.RWMutex
}

func getNewMainWordsCnt(cnt int) *MainWordsCount {
	return &MainWordsCount{cnt, &sync.RWMutex{}}
}

func (mwc *MainWordsCount) Add(cnt int) {
	mwc.Lock()
	mwc.count += cnt
	mwc.Unlock()
}

func (mwc *MainWordsCount) Get() int {
	mwc.RLock()
	defer mwc.RUnlock()
	return mwc.count
}

/* --------------------------- for general manager gorutine ---------------------------- */
var (
	addChan    chan int
	getSumChan chan int
	finish     chan interface{}
)

func monitor() {
	var sumWords int = 0
	for {
		select {
		case addNum := <-addChan:
			sumWords += addNum
		case getSumChan <- sumWords:
		case <-finish:
			close(getSumChan)
			close(addChan)
			return
		}
	}
}

func addToSumm(a int) {
	addChan <- a
}

func getSumm() int {
	return <-getSumChan
}

func main() {

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs)
	go func() {
		for {
			sig := <-sigs
			switch sig {
			case os.Interrupt:
				os.Exit(0)
			case syscall.SIGURG:
			default:
				fmt.Println("\nSygnal skipped. Send SIGINT for ending process:", sig.String())
			}
		}
	}()

	minM := flag.Int("m", 0, "mode of app")
	minH := flag.Bool("h", false, "help")
	workersF := flag.Int("w", 4, "number of count words workers")
	flag.Parse()

	if *minH {
		fmt.Println("Usage of key -m")
		fmt.Println("0: consistent logic without gorutines: files read line by line")
		fmt.Println("1: consistent logic without gorutines: whole file read by ioutils.ReadFile")
		fmt.Println("2: use bufered channel")
		fmt.Println("3: use mutex")
		fmt.Println("4: use mutex whith more gorutines")
		fmt.Println("5: use general manager gorutine")
		os.Exit(0)
	}

	if *minM > 6 || *minM < 0 {
		fmt.Println("Wrong value of prameter (allowed 0, 1, 2, 3, 4, 5  or 6)")
		os.Exit(0)
	}

	if len(flag.Args()) == 0 {
		fmt.Println("Gimme file for counting")
		os.Exit(0)
	}

	re := regexp.MustCompile("[^\\s\\n]+")
	now := time.Now()
	sumWords := 0

	switch *minM {
	case 0:
		fmt.Println("... Use consistent logic without gorutines: files read linie by line")
		for _, filename := range flag.Args() {
			f, err := os.Open(filename)
			if err != nil {
				fmt.Println("Filed to open file:", filename, "error:", err)
				continue
			}

			rd := bufio.NewReader(f)

			fileSum := 0
			finish := false

			for {
				line, err := rd.ReadString('\n')
				if err != nil && err == io.EOF {
					if len(line) > 0 { // if file not ending with '\n'
						finish = true
					} else {
						break
					}
				} else if err != nil {
					fmt.Println("Error reading file:", filename, "error:", err)
					fmt.Println("...Skiping file:", filename)
					break
				}

				fileSum += len(re.FindAllString(line, -1))
				if finish {
					break
				}
			}

			fmt.Printf("Words count in file %s: %d\n", filename, fileSum)
			sumWords += fileSum
		}
	case 1:
		fmt.Println("... Use consistent logic without gorutines: whole file read by ioutils.ReadFile()")
		for _, filename := range flag.Args() {
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				fmt.Println("Filed to open file:", filename, "error:", err)
				continue
			}

			fileSum := len(re.FindAllString(string(data), -1))

			fmt.Printf("Words count in file %s: %d\n", filename, fileSum)
			sumWords += fileSum
		}
	case 2:
		fmt.Println("... Use bufered channel and (-w int) + 1 gorutines for each files")
		var (
			linesCh     map[int]chan string
			countCh     map[int]chan int
			mainCountCh chan int
			wg          *sync.WaitGroup
			wg2         *sync.WaitGroup
			wgCounts    map[int]*sync.WaitGroup
		)

		mainCountCh = make(chan int)
		linesCh = map[int]chan string{}
		countCh = map[int]chan int{}
		wgCounts = map[int]*sync.WaitGroup{}
		wg = &sync.WaitGroup{}
		wg2 = &sync.WaitGroup{}

		for i, filename := range flag.Args() {
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				fmt.Println("Filed to open file:", filename, "error:", err)
				continue
			}

			countCh[i] = make(chan int)

			lines := strings.Split(string(data), "\n")

			linesCh[i] = make(chan string, len(lines))
			for _, line := range lines {
				linesCh[i] <- line
			}
			close(linesCh[i])

			// ---------------------- count words --------------------------
			wgCounts[i] = &sync.WaitGroup{}
			for j := 0; j < *workersF; j++ {
				wgCounts[i].Add(1)
				wg.Add(1)

				go func(in chan string, out chan int, wgLoc *sync.WaitGroup) {
					defer func() {
						wg.Done()
						wgLoc.Done()
					}()

					for line := range in {
						out <- len(re.FindAllString(line, -1))
					}
				}(linesCh[i], countCh[i], wgCounts[i])
			}

			go func(wg *sync.WaitGroup, ch chan int) {
				wg.Wait()
				close(ch)
			}(wgCounts[i], countCh[i])

			// ---------------------- sum words in file --------------------------
			wg.Add(1)
			go func(in chan int, filename string) {
				sum := 0
				defer wg.Done()
				for num := range in {
					sum += num
				}
				fmt.Printf("Words count in file %s: %d\n", filename, sum)
				mainCountCh <- sum
			}(countCh[i], filename)

		}
		// wait and close channel
		go func() {
			wg.Wait()
			close(mainCountCh)
		}()

		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for i2 := range mainCountCh {
				sumWords += i2
			}
		}()

		wg2.Wait()
	case 3:
		fmt.Println("... Use RWMutex, read file line by line and using 3 gorutine per file")

		var (
			mtx        *MainWordsCount
			linesChs   map[int]chan string
			cnWordsChs map[int]chan int
			wg         *sync.WaitGroup
		)

		mtx = getNewMainWordsCnt(0)
		linesChs = map[int]chan string{}
		cnWordsChs = map[int]chan int{}
		wg = &sync.WaitGroup{}

		for i, filename := range flag.Args() {
			f, err := os.Open(filename)
			if err != nil {
				fmt.Println("Filed to open file:", filename, "error:", err)
				continue
			}

			linesChs[i] = make(chan string)
			cnWordsChs[i] = make(chan int)

			rd := bufio.NewReader(f)

			fileSum := 0
			finish := false

			// add file lines to channel
			wg.Add(1)
			go func(out chan<- string) {
				defer func() {
					wg.Done()
					close(out)
				}()

				for {
					line, err := rd.ReadString('\n')
					if err != nil && err == io.EOF {
						if len(line) > 0 { // if file not ending with '\n'
							finish = true
						} else {
							break
						}
					} else if err != nil {
						fmt.Println("Error reading file:", filename, "error:", err)
						fmt.Println("...Skiping file:", filename)
						break
					}

					out <- line

					if finish {
						break
					}
				}
			}(linesChs[i])

			// count words in line
			wg.Add(1)
			go func(in <-chan string, out chan<- int) {
				defer func() {
					wg.Done()
					close(out)
				}()

				for line := range in {
					out <- len(re.FindAllString(line, -1))
				}
			}(linesChs[i], cnWordsChs[i])

			// sum words in file
			wg.Add(1)
			go func(in <-chan int, mtx *MainWordsCount) {
				defer wg.Done()
				fileSum = 0
				for cnt := range in {
					fileSum += cnt
				}
				fmt.Printf("Words count in file %s: %d\n", filename, fileSum)
				mtx.Add(fileSum)
			}(cnWordsChs[i], mtx)

			wg.Wait()
			sumWords = mtx.Get()
		}
	case 4:
		fmt.Println("... Use RWMutex, read file line by line and using (-w int) + 2 gorutine per file")

		var (
			mtx        *MainWordsCount
			linesChs   map[int]chan string
			cnWordsChs map[int]chan int
			wg         *sync.WaitGroup
			wgCounts   map[int]*sync.WaitGroup
		)

		mtx = getNewMainWordsCnt(0)
		linesChs = map[int]chan string{}
		cnWordsChs = map[int]chan int{}
		wg = &sync.WaitGroup{}
		wgCounts = map[int]*sync.WaitGroup{}

		for i, filename := range flag.Args() {
			f, err := os.Open(filename)
			if err != nil {
				fmt.Println("Filed to open file:", filename, "error:", err)
				continue
			}

			linesChs[i] = make(chan string)
			cnWordsChs[i] = make(chan int)

			scaner := bufio.NewScanner(f)

			fileSum := 0
			finish := false

			// ------------------------------------- add file lines to channel -------------------------------------
			wg.Add(1)

			go func(out chan<- string) {
				defer func() {
					wg.Done()
					close(out)
				}()

				for scaner.Scan() {
					line := scaner.Text()
					out <- line

					if finish {
						break
					}
				}
			}(linesChs[i])

			// ------------------------------------- count words in line ----------------------------------------------
			wgCounts[i] = &sync.WaitGroup{}
			for j := 0; j < *workersF; j++ {
				wgCounts[i].Add(1)
				wg.Add(1)

				go func(in <-chan string, out chan<- int) {
					defer func() {
						wg.Done()
						wgCounts[i].Done()
					}()

					for line := range in {
						out <- len(re.FindAllString(line, -1))
					}
				}(linesChs[i], cnWordsChs[i])
			}

			go func(wg *sync.WaitGroup) {
				wg.Wait()
				close(cnWordsChs[i])
			}(wgCounts[i])

			// ------------------------------------- sum words in file -------------------------------------
			wg.Add(1)
			go func(in <-chan int, mtx *MainWordsCount) {
				defer wg.Done()
				fileSum = 0
				for cnt := range in {
					fileSum += cnt
				}
				fmt.Printf("Words count in file %s: %d\n", filename, fileSum)
				mtx.Add(fileSum)
			}(cnWordsChs[i], mtx)

			wg.Wait()
			sumWords = mtx.Get()
		}
	case 5:
		fmt.Println("... Use general gorutine and (-w int) + 2 gorutines for counting + signal channel for closing global channels ")

		var (
			countChs map[int]chan int
			linesChs map[int]chan string
			wgMain   *sync.WaitGroup
			wgCountG map[int]*sync.WaitGroup
		)

		addChan = make(chan int)
		getSumChan = make(chan int)
		wgMain = &sync.WaitGroup{}
		wgCountG = map[int]*sync.WaitGroup{}
		linesChs = map[int]chan string{}
		countChs = map[int]chan int{}
		finish = make(chan interface{})

		go monitor()

		for i, filename := range flag.Args() {
			f, err := os.Open(filename)
			if err != nil {
				fmt.Printf("Can't open file %s. Error: %s\n", filename, err)
				continue
			}

			linesChs[i] = make(chan string)

			wgMain.Add(1)
			scaner := bufio.NewScanner(f)
			go func(sc *bufio.Scanner, out chan<- string) {
				defer wgMain.Done()
				for sc.Scan() {
					out <- sc.Text()
				}
				close(out)
			}(scaner, linesChs[i])

			wgCountG[i] = &sync.WaitGroup{}
			countChs[i] = make(chan int)
			for j := 0; j < *workersF; j++ {
				wgMain.Add(1)
				wgCountG[i].Add(1)
				go func(in chan string, out chan int, wg *sync.WaitGroup) {
					defer func() {
						wg.Done()
						wgMain.Done()
					}()
					for line := range in {
						out <- len(re.FindAllString(line, -1))
					}
				}(linesChs[i], countChs[i], wgCountG[i])
			}

			go func(wg *sync.WaitGroup, closeCh chan int) {
				wg.Wait()
				close(closeCh)
			}(wgCountG[i], countChs[i])

			wgMain.Add(1)
			go func(in <-chan int) {
				defer wgMain.Done()
				fileSum := 0
				for num := range in {
					fileSum += num
				}
				fmt.Printf("Words count in file %s: %d\n", filename, fileSum)
				addToSumm(fileSum)
			}(countChs[i])

			wgMain.Wait()
		}

		sumWords = getSumm()
		finish <- true
	}

	fmt.Printf("Words: %d\n", sumWords)
	fmt.Println("... End of program")
	fmt.Printf("Execution time duration: %.5f seconds\n", time.Since(now).Seconds())
	PrintMemUsage()
}

// ------------------------ service functions ---------------
func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
