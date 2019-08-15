package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
	// "golang.org/x/sys/unix"
)

func main() {
	var buf int
	var count int
	flag.IntVar(&buf, "buffer", 1, "buffer size")
	flag.IntVar(&count, "count", 100, "how many read to do")
	flag.Parse()
	var myfile string
	if len(flag.Args()) >= 1 {
		myfile = flag.Arg(0)
	} else {
		log.Fatalln("no file given!")
	}

	f, err := os.Open(myfile)
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
		log.Println("0-sized file, trying as a block device")
		blkSize := getBlockDeviceSize(f)
		log.Printf("block device size: %v\n", ByteCountDecimal(blkSize))
		if blkSize > 0 {
			size = blkSize
		}
	}
	if size == 0 {
		os.Exit(-1)
	}

	log.Printf("size: %v (%v), doing %v req...", ByteCountDecimal(size), ByteCountBinary(size), count)

	b := make([]byte, buf)
	start := time.Now()
	for index := 0; index < count; index++ {
		myrand := rand.Int63n(size)
		//log.Printf("rand: %v %v", myrand, time.Now().UnixNano())
		_, err = f.ReadAt(b, myrand-1)
		if err != nil {
			log.Fatal(err)
		}
		//log.Println(r)
	}

	t := time.Since(start)
	//log.Printf("total time: %v ns (%v ms) for %v requests", t.Nanoseconds(), t.Nanoseconds()/1000000, count)
	log.Printf("total time: %v ns (%v) for %v requests", t.Nanoseconds(), t.String(), count)
	log.Printf("per rq time: %v ns (%v ms)", t.Nanoseconds()/int64(count), t.Nanoseconds()/1000000/int64(count))
	//log.Printf("timed: %v", (t/time.Duration(count)).Nanoseconds()/100)
	// i, err := unix.IoctlGetInt(int(f.Fd()), unix.BLKGETSIZE64)
	// if err != nil {
	// 	log.Fatal(err)
	// }
}

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
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
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
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
