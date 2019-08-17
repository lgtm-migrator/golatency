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
)

func main() {
	var count int
	var nocache bool
	var seqTest bool
	flag.IntVar(&count, "count", 100, "how many read to do")
	flag.BoolVar(&nocache, "nocache", false, "bypass OS Cache")
	flag.BoolVar(&seqTest, "T", false, "finish by a quick sequential complete file read")
	flag.Parse()
	var myfile string
	if len(flag.Args()) >= 1 {
		myfile = flag.Arg(0)
	} else {
		log.Fatalln("no file given!")
	}

	OpenFile := os.OpenFile
	if nocache {
		log.Println("nocache requested")
		OpenFile = directio.OpenFile
	}
	f, err := OpenFile(myfile, os.O_RDONLY, 0400)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	log.Printf("file %v opened\n", myfile)

	s, err := f.Stat()
	if err != nil {
		log.Fatal(err)
	}
	size := s.Size()

	if size == 0 {
		log.Println("0-sized file, trying to read size as a block device")
		blkSize := getBlockDeviceSize(f)
		log.Printf("found block device size: %v\n", ByteCountDecimal(blkSize))
		if blkSize > 0 {
			size = blkSize
		}
	}
	if size == 0 {
		os.Exit(-1)
	}

	log.Printf("size: %v (%v), doing %v req...", ByteCountDecimal(size), ByteCountBinary(size), count)

	var b []byte
	if nocache {
		b = directio.AlignedBlock(directio.BlockSize)
		if int64(len(b)) > size {
			log.Printf("-nocache needs a file at least %v B long, we will probably fail", len(b))
		}
	} else {
		b = make([]byte, 1)
	}
	// dummy seeding <-- disabled for deterministic seeks
	// rand.Seed(int64(os.Getpid()))
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
			log.Fatal(err)
		}
	}

	t := time.Since(start)
	log.Printf("total time: %v ns (%v) for %v requests", t.Nanoseconds(), t.String(), count)
	durationPerReq, _ := time.ParseDuration(strconv.Itoa(int(t.Nanoseconds()/int64(count))) + "ns")
	log.Printf("per rq time: %v ns (%v)", t.Nanoseconds()/int64(count), durationPerReq.String())
	log.Printf("bytes requested (%v blocks): %v (512) | %v (4096)",
		count,
		ByteCountDecimal(int64(512*count)),
		ByteCountDecimal(int64(4096*count)))

	if seqTest {
		_, err := f.Seek(0, 0)
		if err != nil {
			log.Fatal(err)
		}
		b := make([]byte, 128*1024)
		var total int64 = 0
		var steptotal int64 = 0
		start := time.Now()
		step := start
		log.Println("doing a seq read ...")
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
				fmt.Printf("\r ~ %v/s (%v - %v)\r", ByteCountDecimal(steptotal), ByteCountDecimal(total), ByteCountBinary(total))
				step = time.Now()
				steptotal = 0
			}
		}
		t := time.Since(start)
		log.Printf("%v bytes read in %v (%v/s)", ByteCountDecimal(total), t.Round(100*time.Millisecond).String(), ByteCountDecimal(int64(float64(total)/t.Seconds())))
	}
}

// these 2 are ripped off from the interweb

func ByteCountDecimal(b int64) string {
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

func ByteCountBinary(b int64) string {
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
