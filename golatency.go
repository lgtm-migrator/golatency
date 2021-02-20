package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/directio"

	auroraPackage "github.com/logrusorgru/aurora/v3"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

func main() {
	var (
		routines  int
		count     int
		nocache   bool
		seqTest   bool
		quickTest bool
		seed      bool
	)
	flag.IntVar(&routines, "p", 1, "how many goroutines")
	flag.IntVar(&count, "count", 1000, "how many read to do (per goroutine)")
	flag.BoolVar(&nocache, "nocache", false, "bypass OS Cache")
	flag.BoolVar(&seqTest, "T", false, "finish by a sequential complete file read")
	flag.BoolVar(&quickTest, "t", false, "finish by a quick sequential complete file read (10s max)")
	flag.BoolVar(&seed, "truerandom", false, "seeks are not deterministic (repeatable) anymore")
	flag.Parse()

	aurora := auroraPackage.NewAurora(isatty.IsTerminal(os.Stdout.Fd()))
	log.SetOutput(colorable.NewColorableStdout())
	emphasis := aurora.Green

	var myfile string
	if len(flag.Args()) >= 1 {
		myfile = flag.Arg(0)
	} else {
		log.Fatalln("no file given!")
	}

	OpenFile := os.OpenFile
	if nocache {
		log.Println(emphasis("nocache requested"))
		OpenFile = directio.OpenFile
	}
	f, err := OpenFile(myfile, os.O_RDONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	log.Printf("file %v opened\n", emphasis(myfile))

	s, err := f.Stat()
	if err != nil {
		log.Fatal(err)
	}

	size := s.Size()

	if size == 0 {
		if s.Mode()&os.ModeDevice != 0 && s.Mode()&os.ModeCharDevice == 0 {
			log.Println("0-sized file is reported as a block device, trying to read size as such...")
		} else {
			log.Println("0-sized file, trying to read size as a block device as a last resort...")
		}
		blkSize := getBlockDeviceSize(f)
		log.Printf("found block device size: %v\n", byteCountDecimal(blkSize))
		if blkSize > 0 {
			size = blkSize
		}
	}
	if size == 0 {
		os.Exit(-1)
	}

	log.Printf("size: %v (%v), doing %v req * %v ...", emphasis(byteCountDecimal(size)), byteCountBinary(size), count, routines)

	results := make(chan time.Duration)
	realstart := time.Now()

	for i := 0; i < routines; i++ {
		go func(n int) {
			var b []byte
			if nocache {
				b = directio.AlignedBlock(directio.BlockSize)
				if int64(len(b)) > size {
					if n == 0 {
						log.Printf("(%v) -nocache needs a file at least %v B long, we will probably fail", n, len(b))
					}
				}
			} else {
				b = make([]byte, 1)
			}
			if seed {
				dummySeed := int64(os.Getpid()) + time.Now().UnixNano()
				//log.Printf("seeding is set to %v", emphasis(dummySeed))
				if n == 0 {
					log.Printf("(%v) seeding nonsense as requested.", n)
				}
				rand.Seed(dummySeed)
			} else {
				dummySeed := n
				//log.Printf("seeding is set to %v", emphasis(dummySeed))
				rand.Seed(int64(dummySeed))
			}
			var myrand int64
			start := time.Now()
			for index := 0; index < count; index++ {
				var alignSize int64 = directio.AlignSize
				if nocache && alignSize != 0 {
					// random aligned offset
					myrand = rand.Int63n((size-1)/alignSize) * alignSize
				} else {
					//random offset
					myrand = rand.Int63n(size - 1)
				}
				_, err = f.ReadAt(b, myrand)
				if err != nil {
					log.Printf("(%v) %v", n, err)
				}
			}
			results <- time.Since(start)
		}(i)
	}

	var totaltime time.Duration
	for i := 0; i < routines; i++ {
		totaltime += <-results
	}
	realtime := time.Since(realstart)

	log.Printf("totaltime: %v ns (%s) for %v req/routine (%v goroutines)", totaltime.Nanoseconds(), totaltime, count, routines)
	log.Printf("realtime:  %v ns (%s) for %v req/routine (%v goroutines)", realtime.Nanoseconds(), realtime, count, routines)
	totalDurationPerReq, _ := time.ParseDuration(strconv.Itoa(int(totaltime.Nanoseconds()/int64(routines*count))) + "ns")
	realDurationPerReq, _ := time.ParseDuration(strconv.Itoa(int(realtime.Nanoseconds()/int64(routines*count))) + "ns")
	log.Printf("total per req time: %s", emphasis(totalDurationPerReq))
	log.Printf("real  per req time: %s", emphasis(realDurationPerReq))
	log.Printf("bytes requested (%v blocks): %v (512) | %v (4096)",
		count,
		byteCountDecimal(int64(512*count*routines)),
		byteCountDecimal(int64(4096*count*routines)))

	if seqTest || quickTest {
		_, err := f.Seek(0, 0)
		if err != nil {
			log.Fatal(err)
		}
		b := make([]byte, 128*1024)
		var total int64
		var steptotal int64
		start := time.Now()
		step := start
		if seqTest {
			log.Println("doing a seq read ...")
		} else {
			log.Println("doing a quick seq read ...")
		}
		for {
			n, err := f.Read(b)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			total += int64(n)
			steptotal += int64(n)
			if time.Since(step) >= time.Second {
				fmt.Print(strings.Repeat(" ", 60))
				fmt.Printf("\r ~ %v/s (%v - %v)\r", byteCountDecimal(steptotal), byteCountDecimal(total), byteCountBinary(total))
				step = time.Now()
				steptotal = 0
			}
			if quickTest && time.Since(start) > 10*time.Second {
				break
			}
		}
		t := time.Since(start)
		log.Printf("%v bytes read in %s (%s)",
			byteCountDecimal(total),
			t,
			emphasis(byteCountDecimal(int64(float64(total)/t.Seconds()))+"/s"))
	}
}

// these 2 are ripped off from the interweb

func byteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func byteCountBinary(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
